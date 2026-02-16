// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dhcp

import (
	"fmt"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"golang.org/x/net/ipv4"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// RelayHandler creates a handler that forwards DHCP packets to an upstream server.
// It also snoops on the traffic to populate the lease store (passive mode).
// RelayHandler creates a handler that forwards DHCP packets to an upstream server.
// It also snoops on the traffic to populate the lease store (passive mode).
func (s *Service) createRelayHandler(scope config.DHCPScope, relayTo []string, downstreamConn net.PacketConn, ingressConn net.PacketConn) (func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4), error) {
	if len(relayTo) == 0 {
		return nil, fmt.Errorf("no relay targets specified")
	}

	targets := make([]*net.UDPAddr, 0, len(relayTo))
	for _, target := range relayTo {
		// Default to port 67 if not specified
		if _, _, err := net.SplitHostPort(target); err != nil {
			target = net.JoinHostPort(target, "67")
		}
		addr, err := net.ResolveUDPAddr("udp4", target)
		if err != nil {
			return nil, fmt.Errorf("invalid relay target %s: %w", target, err)
		}
		targets = append(targets, addr)
	}

	logger := logging.WithComponent("dhcp-relay")

	// Start Ingress Loop reading from ingressConn (Upstream Listener)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := ingressConn.ReadFrom(buf)
			if err != nil {
				if s.running {
					logger.WithError(err).Error("Relay ingress read error")
				}
				return
			}

			// Parse packet
			pkt, err := dhcpv4.FromBytes(buf[:n])
			if err != nil {
				logger.WithError(err).Debug("Failed to parse relay ingress packet")
				continue
			}

			// Handle Server Packet (Reply)
			// Filter: only accept packets from source port 67 (server port)
			if udpAddr, ok := addr.(*net.UDPAddr); ok && udpAddr.Port != 67 {
				continue
			}

			logging.WithComponent("dhcp-relay").Debug("Received relay response from upstream", "type", pkt.MessageType(), "from", addr)
			s.handleServerPacket(downstreamConn, pkt, scope, logger)
		}
	}()

	return func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
		// Filter: only accept packets from clients (source port 68) OR if targeted to us (giaddr)
		// For now, simple filter by source port if possible.
		if udpAddr, ok := peer.(*net.UDPAddr); ok && udpAddr.Port != 68 {
			// Possibly a server response arriving on the client-facing socket?
			// Standard relay behavior is to ignore these here as they are handled by the ingress loop.
			return
		}

		switch m.MessageType() {
		case dhcpv4.MessageTypeDiscover, dhcpv4.MessageTypeRequest, dhcpv4.MessageTypeDecline, dhcpv4.MessageTypeRelease, dhcpv4.MessageTypeInform:
			// Client -> Server
			// Pass ingressConn as the "upstream sender" to ensure Source Port 67
			s.handleClientPacket(ingressConn, m, targets, scope, logger)
		case dhcpv4.MessageTypeOffer, dhcpv4.MessageTypeAck, dhcpv4.MessageTypeNak:
			// If it somehow arrives here (e.g. unicast to our GIADDR), handle it.
			s.handleServerPacket(conn, m, scope, logger)
		}
	}, nil
}

func (s *Service) handleClientPacket(conn net.PacketConn, m *dhcpv4.DHCPv4, targets []*net.UDPAddr, scope config.DHCPScope, logger *logging.Logger) {
	// Set GIADDR to our interface IP if not set
	if m.GatewayIPAddr.IsUnspecified() || m.GatewayIPAddr.Equal(net.IPv4zero) {
		// We need the IP of the interface we received this on.
		// Try usage of scope.Router first
		var routerIP net.IP
		if scope.Router != "" {
			routerIP = net.ParseIP(scope.Router)
		}

		// Fallback: Resolve IP from interface name
		if routerIP == nil && scope.Interface != "" {
			if iface, err := net.InterfaceByName(scope.Interface); err == nil {
				addrs, err := iface.Addrs()
				if err == nil {
					for _, addr := range addrs {
						if ipnet, ok := addr.(*net.IPNet); ok {
							if ip4 := ipnet.IP.To4(); ip4 != nil {
								routerIP = ip4
								break
							}
						}
					}
				}
			}
		}

		if routerIP != nil {
			m.GatewayIPAddr = routerIP
		} else {
			logger.Warn("Failed to determine GIADDR (Router IP) for relay", "scope", scope.Name, "interface", scope.Interface)
		}
	}

	// Log packet reception
	logger.Debug("Relaying packet", "type", m.MessageType(), "giaddr", m.GatewayIPAddr, "hops", m.HopCount)

	// Increment Hops
	m.HopCount++
	if m.HopCount > 16 {
		logger.Warn("Dropping packet due to hop count limit", "hops", m.HopCount)
		return // Loop prevention
	}

	// Forward to all targets
	bytes := m.ToBytes()
	for _, target := range targets {
		if _, err := conn.WriteTo(bytes, target); err != nil {
			logger.WithError(err).Warn("Failed to forward to upstream", "target", target)
		} else {
			logger.Debug("Forwarded to upstream", "target", target)
		}
	}
}

func (s *Service) handleServerPacket(conn net.PacketConn, m *dhcpv4.DHCPv4, scope config.DHCPScope, logger *logging.Logger) {
	// Snoop lease information
	if m.MessageType() == dhcpv4.MessageTypeAck {
		go s.snoopLease(m)
	}

	// Forward to client
	destIP := m.YourIPAddr
	destPort := 68

	// Standard relay behavior: if GIADDR is used, the relay must decide how to reach the client.
	// Since the client might not have an IP yet (Discover/Offer/Request/Ack phase), unicast to YIADDR
	// often fails without a raw socket because the host doesn't have an ARP entry.
	// We'll force broadcast to the local link (255.255.255.255:68) for ALL responses to be safe.
	destIP = net.IPv4bcast

	dest := &net.UDPAddr{IP: destIP, Port: destPort}

	logger.Debug("Relaying response to client", "type", m.MessageType(), "dest", dest, "yiaddr", m.YourIPAddr)

	// Determine interface index to force output interface
	var ifIndex int
	if scope.Interface != "" {
		if iface, err := net.InterfaceByName(scope.Interface); err == nil {
			ifIndex = iface.Index
		}
	}

	if ifIndex > 0 {
		// Use ipv4.PacketConn to specify outgoing interface
		pconn := ipv4.NewPacketConn(conn)
		cm := &ipv4.ControlMessage{
			IfIndex: ifIndex,
		}
		if _, err := pconn.WriteTo(m.ToBytes(), cm, dest); err != nil {
			logger.WithError(err).Warn("Failed to forward to client (via ipv4)", "dest", dest, "iface", scope.Interface)
		}
	} else {
		// Fallback to standard WriteTo
		if _, err := conn.WriteTo(m.ToBytes(), dest); err != nil {
			logger.WithError(err).Warn("Failed to forward to client", "dest", dest)
		}
	}
}

func (s *Service) snoopLease(m *dhcpv4.DHCPv4) {
	// Extract info
	mac := m.ClientHWAddr.String()
	ip := m.YourIPAddr
	hostname := m.HostName()

	// Register in our stores/DNS
	// We need to find which store this belongs to?
	// Or just use a global "passive" registration?

	// For now, iterate stores and find one that matches the subnet?
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ls := range s.leaseStores {
		if ls.Subnet != nil && ls.Subnet.Contains(ip) {
			// Found matching subnet store
			// We can bypass Allocate and just persist/cache
			// Use a special method "SnoopLease" on LeaseStore? or just reuse persistLease?
			// Need to verify thread safety if we access LeaseStore directly.
			// LeaseStore has a lock.

			// We need a way to add passive lease.
			// For now, let's assume we can just ignore it or log it,
			// the USER asked to "allow registering the returned IP address in dns".

			// So we need to update DNS.
			if s.dnsUpdater != nil && hostname != "" {
				s.dnsUpdater.AddRecord(hostname, ip)
			}

			// And device registry? (Maybe via leaseListener)
			if s.leaseListener != nil {
				s.leaseListener.OnLease(mac, ip, hostname)
			}

			// We probably should also persist it in the store so it survives reboots
			// and shows up in UI as a "leased" device.
			ls.Lock()
			ls.Leases[mac] = ip
			ls.TakenIPs[ip.String()] = mac
			// Update expiration based on lease time option
			// ...
			ls.Unlock()

			break
		}
	}
}
