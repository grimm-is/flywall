// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dns

import (
	"net"
	"strings"
	"testing"

	"grimm.is/flywall/internal/config"
)

func TestService_PTRRecords(t *testing.T) {
	s := &Service{
		records: make(map[string]config.DNSRecord),
	}

	hostname := "client-1"
	ip := net.ParseIP("192.168.1.50")

	// 1. Test AddRecord creates PTR
	s.AddRecord(hostname, ip)

	fqdn := "client-1."
	ptrName := "50.1.168.192.in-addr.arpa."

	if _, ok := s.records[strings.ToLower(fqdn)]; !ok {
		t.Errorf("A record not found")
	}

	if rec, ok := s.records[strings.ToLower(ptrName)]; !ok {
		t.Errorf("PTR record not found")
	} else if rec.Value != fqdn {
		t.Errorf("Expected PTR value %s, got %s", fqdn, rec.Value)
	}

	// 2. Test RemoveRecord deletes PTR
	s.RemoveRecord(hostname)

	if _, ok := s.records[strings.ToLower(fqdn)]; ok {
		t.Errorf("A record still exists after removal")
	}

	if _, ok := s.records[strings.ToLower(ptrName)]; ok {
		t.Errorf("PTR record still exists after removal")
	}
}

func TestService_PTR_IPv6(t *testing.T) {
	s := &Service{
		records: make(map[string]config.DNSRecord),
	}
	hostname := "ipv6-host"
	ip := net.ParseIP("2001:db8::1")

	s.AddRecord(hostname, ip)

	fqdn := "ipv6-host."
	// Just check if we have an ip6.arpa record pointing to fqdn
	found := false
	for name, rec := range s.records {
		if strings.HasSuffix(name, "ip6.arpa.") && rec.Value == fqdn {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("IPv6 PTR record not created for %s", ip)
	}
}

func TestService_PTR_Overwrite(t *testing.T) {
	s := &Service{
		records: make(map[string]config.DNSRecord),
	}
	hostname := "host-1"
	ip1 := net.ParseIP("192.168.1.10")
	ip2 := net.ParseIP("192.168.1.20")

	// Add first
	s.AddRecord(hostname, ip1)

	// Add second (same hostname, new IP)
	s.AddRecord(hostname, ip2)

	ptrName2 := "20.1.168.192.in-addr.arpa."
	if _, ok := s.records[ptrName2]; !ok {
		t.Errorf("New PTR record not created after overwrite")
	}
}
