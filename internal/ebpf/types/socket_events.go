// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package types

import (
	"net"
	"time"
)

// DNSQueryEvent represents a DNS query event captured by socket filter
type DNSQueryEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	PID          uint32    `json:"pid"`
	TID          uint32    `json:"tid"`
	SourceIP     net.IP    `json:"source_ip"`
	SourcePort   uint16    `json:"source_port"`
	DestIP       net.IP    `json:"dest_ip"`
	DestPort     uint16    `json:"dest_port"`
	QueryID      uint16    `json:"query_id"`
	QueryType    uint16    `json:"query_type"`
	QueryClass   uint16    `json:"query_class"`
	Domain       string    `json:"domain"`
	PacketSize   uint16    `json:"packet_size"`
}

// DNSResponseEvent represents a DNS response event captured by socket filter
type DNSResponseEvent struct {
	Timestamp     time.Time     `json:"timestamp"`
	QueryID       uint16        `json:"query_id"`
	ResponseCode  uint8         `json:"response_code"`
	AnswerCount   uint16        `json:"answer_count"`
	Domain        string        `json:"domain"`
	ResponseTime  time.Duration `json:"response_time"`
	PacketSize    uint16        `json:"packet_size"`
	Answers       []DNSAnswer   `json:"answers,omitempty"`
}

// DNSAnswer represents a DNS answer record
type DNSAnswer struct {
	Name     string `json:"name"`
	Type     uint16 `json:"type"`
	Class    uint16 `json:"class"`
	TTL      uint32 `json:"ttl"`
	Data     string `json:"data"`
	Priority uint16 `json:"priority,omitempty"` // For SRV records
}

// TLSHandshakeEvent represents a TLS handshake event
type TLSHandshakeEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	PID         uint32    `json:"pid"`
	TID         uint32    `json:"tid"`
	SourceIP    net.IP    `json:"source_ip"`
	SourcePort  uint16    `json:"source_port"`
	DestIP      net.IP    `json:"dest_ip"`
	DestPort    uint16    `json:"dest_port"`
	Version     uint16    `json:"version"`     // TLS version
	CipherSuite uint16    `json:"cipher_suite"`
	JA3Hash     string    `json:"ja3_hash,omitempty"`
	JA3         string    `json:"ja3,omitempty"`
	SNI         string    `json:"sni,omitempty"`
	PacketSize  uint16    `json:"packet_size"`
}

// TLSCertificateEvent represents a TLS certificate event
type TLSCertificateEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	PID            uint32    `json:"pid"`
	TID            uint32    `json:"tid"`
	SourceIP       net.IP    `json:"source_ip"`
	SourcePort     uint16    `json:"source_port"`
	DestIP         net.IP    `json:"dest_ip"`
	DestPort       uint16    `json:"dest_port"`
	Certificate    []byte    `json:"certificate,omitempty"`
	CertHash       string    `json:"cert_hash,omitempty"`
	Subject        string    `json:"subject,omitempty"`
	Issuer         string    `json:"issuer,omitempty"`
	SerialNumber   string    `json:"serial_number,omitempty"`
	ValidFrom      time.Time `json:"valid_from,omitempty"`
	ValidUntil     time.Time `json:"valid_until,omitempty"`
	IsSelfSigned  bool      `json:"is_self_signed"`
	IsExpired     bool      `json:"is_expired"`
	IsInvalid     bool      `json:"is_invalid"`
	PacketSize    uint16    `json:"packet_size"`
}

// DHCPDiscoverEvent represents a DHCP discovery event
type DHCPDiscoverEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	PID         uint32    `json:"pid"`
	TID         uint32    `json:"tid"`
	SourceIP    net.IP    `json:"source_ip"`
	SourcePort  uint16    `json:"source_port"`
	DestIP      net.IP    `json:"dest_ip"`
	DestPort    uint16    `json:"dest_port"`
	MACAddress  string    `json:"mac_address"`
	HostName    string    `json:"host_name,omitempty"`
	VendorClass string    `json:"vendor_class,omitempty"`
	PacketSize  uint16    `json:"packet_size"`
}

// DHCPOfferEvent represents a DHCP offer event
type DHCPOfferEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	PID          uint32    `json:"pid"`
	TID          uint32    `json:"tid"`
	SourceIP     net.IP    `json:"source_ip"`
	SourcePort   uint16    `json:"source_port"`
	DestIP       net.IP    `json:"dest_ip"`
	DestPort     uint16    `json:"dest_port"`
	YourIP       net.IP    `json:"your_ip"`
	ServerIP     net.IP    `json:"server_ip"`
	SubnetMask   net.IP    `json:"subnet_mask,omitempty"`
	Router       net.IP    `json:"router,omitempty"`
	DNSServers   []net.IP  `json:"dns_servers,omitempty"`
	LeaseTime    uint32    `json:"lease_time"`
	PacketSize   uint16    `json:"packet_size"`
}

// DHCPRequestEvent represents a DHCP request event
type DHCPRequestEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	PID         uint32    `json:"pid"`
	TID         uint32    `json:"tid"`
	SourceIP    net.IP    `json:"source_ip"`
	SourcePort  uint16    `json:"source_port"`
	DestIP      net.IP    `json:"dest_ip"`
	DestPort    uint16    `json:"dest_port"`
	MACAddress  string    `json:"mac_address"`
	RequestedIP net.IP    `json:"requested_ip,omitempty"`
	ServerIP    net.IP    `json:"server_ip,omitempty"`
	HostName    string    `json:"host_name,omitempty"`
	PacketSize  uint16    `json:"packet_size"`
}

// DHCPAckEvent represents a DHCP acknowledgment event
type DHCPAckEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	PID          uint32    `json:"pid"`
	TID          uint32    `json:"tid"`
	SourceIP     net.IP    `json:"source_ip"`
	SourcePort   uint16    `json:"source_port"`
	DestIP       net.IP    `json:"dest_ip"`
	DestPort     uint16    `json:"dest_port"`
	MACAddress   string    `json:"mac_address"`
	YourIP       net.IP    `json:"your_ip"`
	ServerIP     net.IP    `json:"server_ip"`
	SubnetMask   net.IP    `json:"subnet_mask,omitempty"`
	Router       net.IP    `json:"router,omitempty"`
	DNSServers   []net.IP  `json:"dns_servers,omitempty"`
	LeaseTime    uint32    `json:"lease_time"`
	RenewalTime  uint32    `json:"renewal_time,omitempty"`
	RebindingTime uint32   `json:"rebinding_time,omitempty"`
	PacketSize   uint16    `json:"packet_size"`
}

// DeviceInfo represents discovered device information
type DeviceInfo struct {
	MACAddress    string    `json:"mac_address"`
	IPAddress     net.IP    `json:"ip_address,omitempty"`
	HostName      string    `json:"host_name,omitempty"`
	Vendor        string    `json:"vendor,omitempty"`
	DeviceType    string    `json:"device_type,omitempty"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	LeaseExpiry   *time.Time `json:"lease_expiry,omitempty"`
	DHCPOptions  map[string]interface{} `json:"dhcp_options,omitempty"`
}

// SocketFilterStats represents aggregated socket filter statistics
type SocketFilterStats struct {
	// DNS statistics
	DNSQueriesProcessed    uint64 `json:"dns_queries_processed"`
	DNSResponsesProcessed  uint64 `json:"dns_responses_processed"`
	DNSQueriesBlocked      uint64 `json:"dns_queries_blocked"`
	DNSResponsesBlocked    uint64 `json:"dns_responses_blocked"`

	// TLS statistics
	TLSHandshakesObserved  uint64 `json:"tls_handshakes_observed"`
	TLSCertificatesValid  uint64 `json:"tls_certificates_valid"`
	TLSCertificatesInvalid uint64 `json:"tls_certificates_invalid"`

	// DHCP statistics
	DHCPDiscoversSeen      uint64 `json:"dhcp_discovers_seen"`
	DHCPOffersSeen         uint64 `json:"dhcp_offers_seen"`
	DHCPRequestsSeen       uint64 `json:"dhcp_requests_seen"`
	DHCPAcksSeen           uint64 `json:"dhcp_acks_seen"`
	DevicesDiscovered      uint64 `json:"devices_discovered"`

	// General statistics
	TotalEventsProcessed   uint64 `json:"total_events_processed"`
	TotalEventsBlocked     uint64 `json:"total_events_blocked"`
	PacketsDropped         uint64 `json:"packets_dropped"`
	Errors                 uint64 `json:"errors"`
	LastUpdate             time.Time `json:"last_update"`
}

// DNS constants
const (
	DNSTypeA     = 1   // IPv4 address
	DNSTypeAAAA  = 28  // IPv6 address
	DNSTypeCNAME = 5   // Canonical name
	DNSTypeMX    = 15  // Mail exchange
	DNSTypeTXT   = 16  // Text records
	DNSTypeSRV   = 33  // Service records
	DNSTypePTR   = 12  // Pointer record
	DNSTypeNS    = 2   // Name server
	DNSTypeSOA   = 6   // Start of authority

	DNSClassIN   = 1   // Internet
	DNSClassCS   = 2   // CSNET
	DNSClassCH   = 3   // CHAOS
	DNSClassHS   = 4   // Hesiod

	DNSRCodeNoError    = 0  // No error
	DNSRCodeFormErr    = 1  // Format error
	DNSRCodeServFail   = 2  // Server failure
	DNSRCodeNXDomain   = 3  // Non-existent domain
	DNSRCodeNotImp      = 4  // Not implemented
	DNSRCodeRefused    = 5  // Query refused
	DNSRCodeYXDomain   = 6  // Name exists when it should not
	DNSRCodeYXRRSet    = 7  // RR set exists when it should not
	DNSRCodeNXRRSet    = 8  // RR set that should exist does not
	DNSRCodeNotAuth    = 9  // Server not authoritative for zone
	DNSRCodeNotZone    = 10 // Name not contained in zone
)

// TLS constants
const (
	TLSVersion10 = 0x0301 // TLS 1.0
	TLSVersion11 = 0x0302 // TLS 1.1
	TLSVersion12 = 0x0303 // TLS 1.2
	TLSVersion13 = 0x0304 // TLS 1.3
)

// DHCP constants
const (
	DHCPPortServer = 67
	DHCPPortClient = 68

	DHCPMsgDiscover = 1
	DHCPMsgOffer    = 2
	DHCPMsgRequest  = 3
	DHCPMsgDecline  = 4
	DHCPMsgAck      = 5
	DHCPMsgNack     = 6
	DHCPMsgRelease  = 7
	DHCPMsgInform   = 8

	DHCPOptSubnetMask   = 1
	DHCPOptRouter       = 3
	DHCPOptDNSServer    = 6
	DHCPOptHostName     = 12
	DHCPOptDomainName   = 15
	DHCPOptLeaseTime    = 51
	DHCPOptMessageType  = 53
	DHCPOptServerID     = 54
	DHCPOptRequestedIP  = 50
	DHCPOptRenewalTime  = 58
	DHCPOptRebindingTime = 59
)
