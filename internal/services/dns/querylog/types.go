package querylog

import "time"

// Entry represents a single DNS query log entry
type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	ClientIP   string    `json:"client_ip"`
	Domain     string    `json:"domain"`
	Type       string    `json:"type"`  // A, AAAA, etc.
	RCode      string    `json:"rcode"` // NOERROR, NXDOMAIN
	Upstream   string    `json:"upstream,omitempty"`
	DurationMs int64     `json:"duration_ms"`
	Blocked    bool      `json:"blocked"`
	BlockList  string    `json:"blocklist,omitempty"`
}

// Stats represents aggregated DNS statistics
type Stats struct {
	TotalQueries   int64        `json:"total_queries"`
	BlockedQueries int64        `json:"blocked_queries"`
	TopDomains     []DomainStat `json:"top_domains"`
	TopClients     []ClientStat `json:"top_clients"`
	TopBlocked     []DomainStat `json:"top_blocked"`
}

type DomainStat struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

type ClientStat struct {
	ClientIP string `json:"client_ip"`
	Count    int64  `json:"count"`
}
