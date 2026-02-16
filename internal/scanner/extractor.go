// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package scanner

import (
	"encoding/json"
	"sync"
)

// DeviceFingerprint represents the identity and behavior profile of a device.
type DeviceFingerprint struct {
	IP  string `json:"ip"`
	MAC string `json:"mac,omitempty"`

	// DHCP Fingerprints
	DHCPv4Vendor  string `json:"dhcp_v4_vendor,omitempty"`  // Option 60
	DHCPv4Params  string `json:"dhcp_v4_params,omitempty"`  // Option 55 (Hex)
	DHCPv6Vendor  string `json:"dhcp_v6_vendor,omitempty"`  // Option 16
	DHCPv6Options string `json:"dhcp_v6_options,omitempty"` // Option 6 (Hex)

	// mDNS Fingerprints
	MDNSNames    []string `json:"mdns_names,omitempty"`
	MDNSServices []string `json:"mdns_services,omitempty"`

	// TLS Fingerprints
	JA3Hashes  []string `json:"ja3_hashes,omitempty"`
	SNIDomains []string `json:"sni_domains,omitempty"`

	// Flow Statistics (Passive)
	// We aggregate these to match conntrack capabilities
	FlowsSeen       int64   `json:"flows_seen"`
	TotalBytesOut   uint64  `json:"total_bytes_out"`
	TotalBytesIn    uint64  `json:"total_bytes_in"`
	AvgFlowDuration float64 `json:"avg_flow_duration"`

	mu sync.Mutex
}

// NewDeviceFingerprint creates a new fingerprint record
func NewDeviceFingerprint(ip string) *DeviceFingerprint {
	return &DeviceFingerprint{
		IP:           ip,
		MDNSNames:    make([]string, 0),
		MDNSServices: make([]string, 0),
		JA3Hashes:    make([]string, 0),
		SNIDomains:   make([]string, 0),
	}
}

// AddMDNS adds a unique mDNS name or service
func (f *DeviceFingerprint) AddMDNS(name, service string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if name != "" && !contains(f.MDNSNames, name) {
		f.MDNSNames = append(f.MDNSNames, name)
	}
	if service != "" && !contains(f.MDNSServices, service) {
		f.MDNSServices = append(f.MDNSServices, service)
	}
}

// AddTLS adds unique JA3 and SNI
func (f *DeviceFingerprint) AddTLS(ja3, sni string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if ja3 != "" && !contains(f.JA3Hashes, ja3) {
		f.JA3Hashes = append(f.JA3Hashes, ja3)
	}
	if sni != "" && !contains(f.SNIDomains, sni) {
		f.SNIDomains = append(f.SNIDomains, sni)
	}
}

// UpdateStats updates flow statistics
func (f *DeviceFingerprint) UpdateStats(bytesOut, bytesIn uint64, duration float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Running average for duration
	f.AvgFlowDuration = (f.AvgFlowDuration*float64(f.FlowsSeen) + duration) / float64(f.FlowsSeen+1)

	f.FlowsSeen++
	f.TotalBytesOut += bytesOut
	f.TotalBytesIn += bytesIn
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ToJSON returns the JSON representation
func (f *DeviceFingerprint) ToJSON() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return json.MarshalIndent(f, "", "  ")
}
