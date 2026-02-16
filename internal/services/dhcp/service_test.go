// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dhcp

import (
	"net"
	"testing"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/state"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/miekg/dns"
)

// mockDNSUpdater implements DNSUpdater for testing
type mockDNSUpdater struct {
	records map[string]net.IP
}

func (m *mockDNSUpdater) AddRecord(name string, ip net.IP) {
	if m.records == nil {
		m.records = make(map[string]net.IP)
	}
	m.records[dns.Fqdn(name)] = ip
}

func (m *mockDNSUpdater) RemoveRecord(name string) {
	if m.records != nil {
		delete(m.records, dns.Fqdn(name))
	}
}

func TestDHCPFlow(t *testing.T) {
	// 1. Setup Service
	store, err := state.NewSQLiteStore(state.DefaultOptions(":memory:"))
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	dnsUpdater := &mockDNSUpdater{}
	svc := NewService(dnsUpdater, store)

	// Create a test scope
	scope := config.DHCPScope{
		Name:       "test-scope",
		Interface:  "eth0",
		RangeStart: "192.168.1.100",
		RangeEnd:   "192.168.1.200",
		Router:     "192.168.1.1",
		DNS:        []string{"8.8.8.8", "8.8.4.4"},
		Domain:     "lan",
		LeaseTime:  "1h",
	}

	// Initialize server instance manually (since we can't bind sockets in unit test easily)
	// We'll test handleDiscover and handleRequest directly using internal logic helper if possible,
	// or we can test `createServer` and use the handler it creates?
	// `createServer` tries to bind sockets. We should probably extract the logic or mock net.PacketConn.
	// But `createServer` returns a handler function. We can use that!
	// It relies on `server4.NewIPv4UDPConn` which might fail without root/capabilities.
	// Let's modify `createServer` to be testable or test `handleDiscover`/`handleRequest` directly.
	// Those are not exported. But they are in the same package, so we can access them in `service_test.go` (package dhcp).

	// We need a LeaseStore
	srv, ls, err := svc.createServer(scope, nil)
	if err == nil {
		// If it succeeded (maybe we have permission?), great. But likely it failed on socket bind.
		_ = srv
	}
	// If createServer failed due to socket, we can still construct a LeaseStore manually for testing the handlers.
	// Replicating createServer logic for LeaseStore:
	startIP := net.ParseIP(scope.RangeStart).To4()
	endIP := net.ParseIP(scope.RangeEnd).To4()
	routerIP := net.ParseIP(scope.Router).To4()
	ls = &LeaseStore{
		Leases:       make(map[string]net.IP),
		TakenIPs:     make(map[string]string),
		Reservations: make(map[string]config.DHCPReservation),
		ReservedIPs:  make(map[string]string),
		RangeStart:   startIP,
		RangeEnd:     endIP,
		// RangeStart is exported, rangeStart was typo
	}
	ls.Subnet = &net.IPNet{
		IP:   routerIP.Mask(net.IPv4Mask(255, 255, 255, 0)),
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}
	ls.leaseTime = 1 * time.Hour

	// 2. Simulate DHCPDISCOVER
	macAddr, _ := net.ParseMAC("00:11:22:33:44:55")
	discover, _ := dhcpv4.NewDiscovery(macAddr, dhcpv4.WithRequestedOptions(dhcpv4.OptionDomainNameServer, dhcpv4.OptionRouter, dhcpv4.OptionDomainName))

	// Handle Discover
	offer, err := handleDiscover(discover, ls, scope, routerIP, nil)
	if err != nil {
		t.Fatalf("handleDiscover failed: %v", err)
	}

	// Verify OFFER
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Errorf("Expected OFFER, got %v", offer.MessageType())
	}
	if !offer.YourIPAddr.Equal(net.ParseIP("192.168.1.100")) {
		t.Errorf("Expected IP 192.168.1.100, got %v", offer.YourIPAddr)
	}

	// Verify Options
	router := offer.Options.Get(dhcpv4.OptionRouter)
	if len(router) == 0 || !net.IP(router).Equal(routerIP) {
		t.Errorf("Router option missing or incorrect: %v", router)
	}

	domain := offer.Options.Get(dhcpv4.GenericOptionCode(119)) // Option 119 (Domain Search)
	if len(domain) == 0 {
		// Fallback to check Option 15 if 119 wasn't set?
		domain = offer.Options.Get(dhcpv4.OptionDomainName)
	}

	// 3. Simulate DHCPREQUEST
	req, _ := dhcpv4.NewRequestFromOffer(offer, dhcpv4.WithRequestedOptions(dhcpv4.OptionDomainNameServer, dhcpv4.OptionRouter, dhcpv4.OptionDomainName))

	// Handle Request
	ackresp, err := handleRequest(req, ls, scope, routerIP, dnsUpdater, nil, nil)
	if err != nil {
		t.Fatalf("handleRequest failed: %v", err)
	}

	// Verify ACK
	if ackresp.MessageType() != dhcpv4.MessageTypeAck {
		t.Errorf("Expected ACK, got %v", ackresp.MessageType())
	}
	if !ackresp.YourIPAddr.Equal(net.ParseIP("192.168.1.100")) {
		t.Errorf("Expected IP 192.168.1.100, got %v", ackresp.YourIPAddr)
	}

	// Verify Lease Persistence
	if ip, ok := ls.Leases[macAddr.String()]; !ok || !ip.Equal(net.ParseIP("192.168.1.100")) {
		t.Errorf("Lease not persisted in store")
	}

	// Verify DNS Update
	// requestWithHost
	requestWithHost, _ := dhcpv4.NewRequestFromOffer(offer, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.OptionHostName, []byte("test-pc"))))
	ackWithHost, _ := handleRequest(requestWithHost, ls, scope, routerIP, dnsUpdater, nil, nil)
	if ackWithHost == nil {
		t.Fatal("handleRequest with host failed")
	}

	// Check DNS
	if ip, ok := dnsUpdater.records["test-pc.lan."]; !ok || !ip.Equal(net.ParseIP("192.168.1.100")) {
		t.Errorf("DNS record not updated for test-pc.lan.: got %v", dnsUpdater.records)
	}
}
