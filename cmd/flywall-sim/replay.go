package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"grimm.is/flywall/internal/clock"
	"grimm.is/flywall/internal/events"
	"grimm.is/flywall/internal/kernel"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/scanner"
	dhcpSvc "grimm.is/flywall/internal/services/dhcp"
)

// Replay processes a PCAP file.
func (r *Replayer) Replay(path string) error {
	handle, err := pcap.OpenOffline(path)
	if err != nil {
		return fmt.Errorf("failed to open PCAP: %w", err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	count := 0
	start := time.Now()

	log.Printf("Starting replay of %s...", path)

	for packet := range packetSource.Packets() {
		// Update simulation clock to packet timestamp
		if packet.Metadata() != nil {
			r.clock.Set(packet.Metadata().Timestamp)
		}

		result := r.ProcessPacket(packet)
		if result.Accepted {
			// Count accepted?
		}
		count++

		if count%1000 == 0 {
			fmt.Printf("\rProcessed %d packets...", count)
		}
	}
	fmt.Printf("\rProcessed %d packets in %v\n", count, time.Since(start))
	return nil
}

// Replayer handles PCAP replay with discovery and learning integration.
type Replayer struct {
	kernel   *kernel.SimKernel
	engine   *learning.Engine
	clock    *clock.MockClock
	eventHub *events.Hub

	// Discovery tracking
	discoveredMACs map[string]bool
	discoveredIPs  map[string]string // IP -> MAC
}

// NewReplayer creates a new replayer instance.
func NewReplayer(k *kernel.SimKernel, e *learning.Engine, clk *clock.MockClock) *Replayer {
	return &Replayer{
		kernel:         k,
		engine:         e,
		clock:          clk,
		eventHub:       events.NewHub(),
		discoveredMACs: make(map[string]bool),
		discoveredIPs:  make(map[string]string),
	}
}

// ProcessPacket handles a single packet from the PCAP.
// It extracts discovery data and updates the kernel state.
func (r *Replayer) ProcessPacket(packet gopacket.Packet) *PacketResult {
	result := &PacketResult{}

	// Extract Ethernet layer for MAC
	var srcMAC string
	if eth := packet.Layer(layers.LayerTypeEthernet); eth != nil {
		srcMAC = eth.(*layers.Ethernet).SrcMAC.String()
	}

	// Process DHCP discovery
	if dhcpEvent := r.extractDHCP(packet); dhcpEvent != nil {
		result.DHCPEvent = dhcpEvent
		r.publishDeviceSeen(dhcpEvent.ClientMAC, "", dhcpEvent.Hostname, "dhcp")

		// Infer OS from fingerprint
		if dhcpEvent.Fingerprint != "" {
			if os := dhcpSvc.InferDeviceOS(dhcpEvent.Fingerprint); os != "" {
				result.InferredOS = os
			}
		}
	}

	// Process mDNS discovery
	fp := scanner.NewDeviceFingerprint("")
	scanner.ExtractMDNS(packet, fp)
	if len(fp.MDNSServices) > 0 || len(fp.MDNSNames) > 0 {
		result.MDNSServices = fp.MDNSServices
		hostname := ""
		if len(fp.MDNSNames) > 0 {
			hostname = fp.MDNSNames[0]
		}
		if srcMAC != "" {
			r.publishDeviceSeen(srcMAC, "", hostname, "mdns")
		}
	}

	// Process ARP for MAC/IP mapping
	if arpEvent := r.extractARP(packet); arpEvent != nil {
		result.ARPMapping = arpEvent
		r.discoveredIPs[arpEvent.IP] = arpEvent.MAC
	}

	// Inject into kernel for flow tracking
	// For simulation, we assume accepted unless kernel blocks it (blocklist)
	// OR learning engine denies it
	kernelAccepted := r.kernel.InjectPacket(packet)

	// Process through Learning Engine for verdict
	pktInfo := r.toPacketInfo(packet)
	if pktInfo != nil {
		verdict, err := r.engine.ProcessPacket(pktInfo)
		if err != nil {
			// e.g. packet info required - ignore
			result.Accepted = kernelAccepted
		} else {
			// Engine verdict overrides kernel if false (blocking)
			// But if kernel blocks (blocklist), it's blocked.
			result.Accepted = kernelAccepted && verdict
		}
	} else {
		result.Accepted = kernelAccepted
	}

	return result
}

// toPacketInfo converts gopacket to learning.PacketInfo
func (r *Replayer) toPacketInfo(packet gopacket.Packet) *learning.PacketInfo {
	// Extract network layer
	var srcIP, dstIP net.IP
	var protocol string

	if ipv4 := packet.Layer(layers.LayerTypeIPv4); ipv4 != nil {
		ip := ipv4.(*layers.IPv4)
		srcIP = ip.SrcIP
		dstIP = ip.DstIP
	} else if ipv6 := packet.Layer(layers.LayerTypeIPv6); ipv6 != nil {
		ip := ipv6.(*layers.IPv6)
		srcIP = ip.SrcIP
		dstIP = ip.DstIP
	} else {
		return nil
	}

	// Extract transport layer
	var dstPort uint16
	if tcp := packet.Layer(layers.LayerTypeTCP); tcp != nil {
		dstPort = uint16(tcp.(*layers.TCP).DstPort)
		protocol = "tcp"
	} else if udp := packet.Layer(layers.LayerTypeUDP); udp != nil {
		dstPort = uint16(udp.(*layers.UDP).DstPort)
		protocol = "udp"
	} else if packet.Layer(layers.LayerTypeICMPv4) != nil {
		protocol = "icmp"
	} else {
		return nil
	}

	// Get MAC from Ethernet
	var srcMAC string
	if eth := packet.Layer(layers.LayerTypeEthernet); eth != nil {
		srcMAC = eth.(*layers.Ethernet).SrcMAC.String()
	}

	return &learning.PacketInfo{
		SrcMAC:   srcMAC,
		SrcIP:    srcIP.String(),
		DstIP:    dstIP.String(),
		DstPort:  int(dstPort),
		Protocol: protocol,
		Policy:   "lan_wan", // Assumption for simulation
	}
}

// extractDHCP tries to parse a DHCP packet.
func (r *Replayer) extractDHCP(packet gopacket.Packet) *dhcpSvc.SnifferEvent {
	// Check for UDP port 67 or 68
	udp := packet.Layer(layers.LayerTypeUDP)
	if udp == nil {
		return nil
	}
	u := udp.(*layers.UDP)
	if u.SrcPort != 67 && u.SrcPort != 68 && u.DstPort != 67 && u.DstPort != 68 {
		return nil
	}

	// Extract application layer
	appLayer := packet.ApplicationLayer()
	if appLayer == nil {
		return nil
	}

	// Parse DHCP
	dhcpPkt, err := dhcpv4.FromBytes(appLayer.Payload())
	if err != nil {
		return nil
	}

	// Only process client requests
	if dhcpPkt.OpCode != dhcpv4.OpcodeBootRequest {
		return nil
	}

	// Use existing DHCP parser
	event := dhcpSvc.ExtractEvent(dhcpPkt, "sim0", nil)
	return &event
}

// ARPEvent represents an ARP MAC/IP mapping.
type ARPEvent struct {
	MAC string
	IP  string
}

// extractARP parses ARP packets for MAC/IP discovery.
func (r *Replayer) extractARP(packet gopacket.Packet) *ARPEvent {
	arp := packet.Layer(layers.LayerTypeARP)
	if arp == nil {
		return nil
	}
	a := arp.(*layers.ARP)

	// Only process ARP replies and requests with valid sender info
	if len(a.SourceHwAddress) < 6 || len(a.SourceProtAddress) < 4 {
		return nil
	}

	mac := net.HardwareAddr(a.SourceHwAddress).String()
	ip := net.IP(a.SourceProtAddress).String()

	// Skip invalid entries
	if ip == "0.0.0.0" || mac == "00:00:00:00:00:00" {
		return nil
	}

	return &ARPEvent{MAC: mac, IP: ip}
}

// publishDeviceSeen publishes a device discovery event.
func (r *Replayer) publishDeviceSeen(mac, ip, hostname, method string) {
	if mac == "" {
		return
	}

	// Track discovery
	isNew := !r.discoveredMACs[mac]
	r.discoveredMACs[mac] = true

	if isNew {
		log.Printf("   🔍 Discovered device: %s (%s) via %s", mac, hostname, method)
	}

	r.eventHub.Publish(events.Event{
		Type:      events.EventDeviceSeen,
		Timestamp: r.clock.Now(),
		Source:    "sim",
		Data: events.DeviceSeenData{
			MAC:      mac,
			IP:       ip,
			Hostname: hostname,
			Method:   method,
		},
	})
}

// PacketResult holds the results of processing a single packet.
type PacketResult struct {
	Accepted     bool
	DHCPEvent    *dhcpSvc.SnifferEvent
	MDNSServices []string
	ARPMapping   *ARPEvent
	InferredOS   string
}

// DiscoveryStats returns statistics about discovered devices.
func (r *Replayer) DiscoveryStats() DiscoveryStats {
	return DiscoveryStats{
		UniqueMACs: len(r.discoveredMACs),
		UniqueIPs:  len(r.discoveredIPs),
	}
}

// DiscoveryStats holds device discovery statistics.
type DiscoveryStats struct {
	UniqueMACs int `json:"unique_macs"`
	UniqueIPs  int `json:"unique_ips"`
}
