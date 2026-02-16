// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package types

import (
	"fmt"
	"net"
)

// FlowKey represents a unique flow identifier
type FlowKey struct {
	SrcIP   uint32  `json:"src_ip"`
	DstIP   uint32  `json:"dst_ip"`
	SrcPort uint16  `json:"src_port"`
	DstPort uint16  `json:"dst_port"`
	IPProto uint8   `json:"ip_proto"`
	_       [3]byte // Padding to match C struct
}

// Hash returns a hash of the flow key for caching
func (fk *FlowKey) Hash() uint64 {
	// Simple hash function - combine fields
	h := uint64(fk.SrcIP)*31 + uint64(fk.DstIP)
	h = h*31 + uint64(fk.SrcPort)
	h = h*31 + uint64(fk.DstPort)
	h = h*31 + uint64(fk.IPProto)
	return h
}

// String returns a string representation of the flow key
func (fk *FlowKey) String() string {
	return fmt.Sprintf("%s:%d->%s:%d proto=%d",
		int2ip(fk.SrcIP), fk.SrcPort,
		int2ip(fk.DstIP), fk.DstPort,
		fk.IPProto)
}

// FlowState represents the state of a flow
type FlowState struct {
	Verdict     uint8     `json:"verdict"`
	QoSProfile  uint8     `json:"qos_profile"`
	Flags       uint16    `json:"flags"`
	PacketCount uint64    `json:"packet_count"`
	ByteCount   uint64    `json:"byte_count"`
	LastSeen    uint64    `json:"last_seen"`
	CreatedAt   uint64    `json:"created_at"`
	ExpiresAt   uint64    `json:"expires_at"`
	JA3Hash     [4]uint32 `json:"ja3_hash"`
	SNI         [64]byte  `json:"sni"`
}

// DHCPKey represents the key for DHCP transaction maps
type DHCPKey struct {
	XID     uint32  `json:"xid"`
	MACAddr [6]byte `json:"mac_addr"`
	_       [2]byte // Padding for alignment
}

// DHCPDiscoverInfo represents DHCP discover information
type DHCPDiscoverInfo struct {
	MACAddr        [6]byte  `json:"mac_addr"`
	HostnameLen    uint8    `json:"hostname_len"`
	Hostname       [64]byte `json:"hostname"`
	VendorClassLen uint8    `json:"vendor_class_len"`
	VendorClass    [64]byte `json:"vendor_class"`
	PacketSize     uint16   `json:"packet_size"`
	_              [6]byte  // Padding
	Timestamp      uint64   `json:"timestamp"`
}

// DHCPOfferInfo represents DHCP offer information
type DHCPOfferInfo struct {
	YourIP     uint32    `json:"your_ip"`
	ServerIP   uint32    `json:"server_ip"`
	SubnetMask uint32    `json:"subnet_mask"`
	Router     uint32    `json:"router"`
	DNSServers [4]uint32 `json:"dns_servers"`
	LeaseTime  uint32    `json:"lease_time"`
	PacketSize uint16    `json:"packet_size"`
	_          [2]byte   // Padding
	Timestamp  uint64    `json:"timestamp"`
}

// DHCPRequestInfo represents DHCP request information
type DHCPRequestInfo struct {
	MACAddr     [6]byte  `json:"mac_addr"`
	RequestedIP uint32   `json:"requested_ip"`
	ServerIP    uint32   `json:"server_ip"`
	HostnameLen uint8    `json:"hostname_len"`
	Hostname    [64]byte `json:"hostname"`
	PacketSize  uint16   `json:"packet_size"`
	_           [6]byte  // Padding
	Timestamp   uint64   `json:"timestamp"`
}

// DHCPAckInfo represents DHCP acknowledge information
type DHCPAckInfo struct {
	YourIP        uint32    `json:"your_ip"`
	ServerIP      uint32    `json:"server_ip"`
	SubnetMask    uint32    `json:"subnet_mask"`
	Router        uint32    `json:"router"`
	DNSServers    [4]uint32 `json:"dns_servers"`
	LeaseTime     uint32    `json:"lease_time"`
	RenewalTime   uint32    `json:"renewal_time"`
	RebindingTime uint32    `json:"rebinding_time"`
	PacketSize    uint16    `json:"packet_size"`
	_             [6]byte   // Padding
	Timestamp     uint64    `json:"timestamp"`
}

// TLSKey represents the key for TLS handshake maps
type TLSKey struct {
	SrcIP   uint32 `json:"src_ip"`
	DstIP   uint32 `json:"dst_ip"`
	SrcPort uint16 `json:"src_port"`
	DstPort uint16 `json:"dst_port"`
}

// TLSHandshakeInfo represents TLS handshake information
type TLSHandshakeInfo struct {
	Version     uint16    `json:"version"`
	CipherSuite uint16    `json:"cipher_suite"`
	SNI         [64]byte  `json:"sni"`
	JA3Hash     [4]uint32 `json:"ja3_hash"`
	Timestamp   uint64    `json:"timestamp"`
}

// Event represents a generic event from the ring buffer
type Event struct {
	Type      uint32    `json:"type"`
	Timestamp uint64    `json:"timestamp"`
	SrcIP     uint32    `json:"src_ip"`
	DstIP     uint32    `json:"dst_ip"`
	SrcPort   uint16    `json:"src_port"`
	DstPort   uint16    `json:"dst_port"`
	Protocol  uint8     `json:"protocol"`
	DataLen   uint8     `json:"data_len"`
	Data      [128]byte `json:"data"` // Raw data union
}

// Statistics represents global statistics
type Statistics struct {
	PacketsProcessed uint64 `json:"packets_processed"`
	PacketsDropped   uint64 `json:"packets_dropped"`
	PacketsPassed    uint64 `json:"packets_passed"`
	BytesProcessed   uint64 `json:"bytes_processed"`
	BlockedIPs       uint64 `json:"blocked_ips"`
	BlockedDNS       uint64 `json:"blocked_dns"`
	FlowsOffloaded   uint64 `json:"flows_offloaded"`
	EventsGenerated  uint64 `json:"events_generated"`
	LastCleanup      uint64 `json:"last_cleanup"`
}

// Verdict constants
const (
	VerdictUnknown uint8 = 0
	VerdictTrusted uint8 = 1
	VerdictDrop    uint8 = 2
)

// Flow state flags
const (
	FlowFlagNone      uint16 = 0
	FlowFlagOffloaded uint16 = 1 << 0
	FlowFlagMonitored uint16 = 1 << 1
	FlowFlagBlocked   uint16 = 1 << 2
)

// QoS profile constants
const (
	QoSProfileDefault     uint8 = 0
	QoSProfileBulk        uint8 = 1
	QoSProfileInteractive uint8 = 2
	QoSProfileVideo       uint8 = 3
	QoSProfileVoice       uint8 = 4
	QoSProfileCritical    uint8 = 5
)

// QoSProfile represents a QoS configuration
type QoSProfile struct {
	RateLimit  uint32 `json:"rate_limit"`
	BurstLimit uint32 `json:"burst_limit"`
	Priority   uint8  `json:"priority"`
	AppClass   uint8  `json:"app_class"`
}

// int2ip converts uint32 to IP string
func int2ip(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ip>>24),
		byte(ip>>16&0xff),
		byte(ip>>8&0xff),
		byte(ip&0xff),
	)
}

// ToNetFlow converts FlowKey to a net.TCPAddr or net.UDPAddr
func (f FlowKey) ToNetFlow() (net.Addr, error) {
	ip := net.IPv4(
		byte(f.SrcIP>>24),
		byte(f.SrcIP>>16&0xff),
		byte(f.SrcIP>>8&0xff),
		byte(f.SrcIP&0xff),
	)

	switch f.IPProto {
	case 6: // TCP
		return &net.TCPAddr{
			IP:   ip,
			Port: int(f.SrcPort),
		}, nil
	case 17: // UDP
		return &net.UDPAddr{
			IP:   ip,
			Port: int(f.SrcPort),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %d", f.IPProto)
	}
}

// IsExpired checks if the flow has expired based on current time
func (f FlowState) IsExpired(now uint64) bool {
	// Use a default timeout of 5 minutes if not explicitly set
	timeout := uint64(300 * 1000000000) // 5 minutes in nanoseconds
	return now > (f.LastSeen + timeout)
}

// UpdateActivity updates the flow's activity timestamp
func (f *FlowState) UpdateActivity(now uint64) {
	f.LastSeen = now
}

// AddPacket adds packet statistics to the flow
func (f *FlowState) AddPacket(bytes uint64, now uint64) {
	f.PacketCount++
	f.ByteCount += bytes
	f.LastSeen = now
}

// HookConfig represents configuration for attaching a hook
type HookConfig struct {
	ProgramName string      `json:"program_name"`
	ProgramType ProgramType `json:"program_type"`
	AttachPoint string      `json:"attach_point"`
	AutoReplace bool        `json:"auto_replace"`
}

// HookStats represents statistics for a hook
type HookStats struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	AttachPoint string `json:"attach_point"`
	AttachedAt  int64  `json:"attached_at"`
	RunCount    uint64 `json:"run_count"`
}

// ProgramType represents different eBPF program types
type ProgramType string

const (
	ProgramTypeXDP          ProgramType = "xdp"
	ProgramTypeTC           ProgramType = "tc"
	ProgramTypeSocketFilter ProgramType = "socket_filter"
	ProgramTypeKprobe       ProgramType = "kprobe"
	ProgramTypeUnspec       ProgramType = "unspec"
)

func (p ProgramType) String() string {
	return string(p)
}

// EventType represents different types of eBPF events
type EventType string

const (
	EventTypeFlowCreated   EventType = "flow_created"
	EventTypeFlowUpdated   EventType = "flow_updated"
	EventTypeFlowExpired   EventType = "flow_expired"
	EventTypeDNSQuery      EventType = "dns_query"
	EventTypeDNSResponse   EventType = "dns_response"
	EventTypeTLSHandshake  EventType = "tls_handshake"
	EventTypeDHCPDiscovery EventType = "dhcp_discovery"
	EventTypeDHCPOffer     EventType = "dhcp_offer"
	EventTypeAlert         EventType = "alert"
	EventTypeStats         EventType = "stats"
)

// DHCPEvent matches the C struct in dhcp_socket.c
type DHCPEvent struct {
	Timestamp      uint64    `json:"timestamp"`
	PID            uint32    `json:"pid"`
	TID            uint32    `json:"tid"`
	SrcIP          uint32    `json:"src_ip"`
	DstIP          uint32    `json:"dst_ip"`
	SrcPort        uint16    `json:"src_port"`
	DstPort        uint16    `json:"dst_port"`
	EventType      uint8     `json:"event_type"`
	_              [3]byte   // Padding
	XID            uint32    `json:"xid"`
	MACAddress     [6]byte   `json:"mac_addr"`
	_              [2]byte   // Padding
	YourIP         uint32    `json:"your_ip"`
	ServerIP       uint32    `json:"server_ip"`
	SubnetMask     uint32    `json:"subnet_mask"`
	Router         uint32    `json:"router"`
	DNSServers     [4]uint32 `json:"dns_servers"`
	LeaseTime      uint32    `json:"lease_time"`
	RenewalTime    uint32    `json:"renewal_time"`
	RebindingTime  uint32    `json:"rebinding_time"`
	RequestedIP    uint32    `json:"requested_ip"`
	HostNameLen    uint8     `json:"hostname_len"`
	HostName       [64]byte  `json:"hostname"`
	VendorClassLen uint8     `json:"vendor_class_len"`
	VendorClass    [64]byte  `json:"vendor_class"`
	PacketSize     uint16    `json:"packet_size"`
	_              [2]byte   // Padding
}
