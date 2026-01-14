package scanner

import (
	"strings"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// ExtractMDNS analyzes a packet for mDNS fingerprints
func ExtractMDNS(packet gopacket.Packet, record *DeviceFingerprint) {
	// mDNS is UDP 5353
	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return
	}
	udp, _ := udpLayer.(*layers.UDP)
	
	if udp.SrcPort != 5353 && udp.DstPort != 5353 {
		return
	}

	// Parse DNS
	if dnsLayer := packet.Layer(layers.LayerTypeDNS); dnsLayer != nil {
		dns, _ := dnsLayer.(*layers.DNS)
		
		// Look at Answers (advertising services) or Additional records
		// Devices often announce themselves in response to queries or spontaneously
		extractDNSRecords(dns.Answers, record)
		extractDNSRecords(dns.Authorities, record)
		extractDNSRecords(dns.Additionals, record)
	}
}

func extractDNSRecords(records []layers.DNSResourceRecord, record *DeviceFingerprint) {
	for _, rr := range records {
		name := string(rr.Name)
		
		// Capture Service Types (PTR records often point to service types like _http._tcp.local)
		if rr.Type == layers.DNSTypePTR {
			// The Name is usually the service type (e.g. _services._dns-sd._udp.local)
			// The PTR data points to the instance name
			// But for fingerprinting, the SERVICE TYPE is most valuable (e.g. _googlecast._tcp)
			
			// If the name starts with _, it's likely a service signature
			if strings.HasPrefix(name, "_") {
				// Clean up trailing dots
				cleanName := strings.TrimSuffix(name, ".")
				record.AddMDNS("", cleanName)
			}
		}
		
		// Capture instance names from SRV records
		if rr.Type == layers.DNSTypeSRV {
			// This is usually the specific device name "Living Room TV._googlecast._tcp.local"
			cleanName := strings.TrimSuffix(name, ".")
			record.AddMDNS(cleanName, "")
		}
	}
}
