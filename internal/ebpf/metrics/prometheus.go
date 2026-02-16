// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all eBPF Prometheus metrics
type Metrics struct {
	// General eBPF metrics
	PacketsProcessed prometheus.Counter
	PacketsDropped   prometheus.Counter
	PacketsPassed    prometheus.Counter
	BytesProcessed   prometheus.Counter

	// Feature-specific metrics
	Features *FeatureMetrics

	// Map metrics
	MapEntries *prometheus.GaugeVec
	MapUpdates *prometheus.CounterVec

	// Hook metrics
	HookAttached *prometheus.GaugeVec
	HookErrors   *prometheus.CounterVec

	// DNS blocklist metrics
	DNSBlocked     prometheus.Counter
	DNSQueries     prometheus.Counter
	DNSDomainCount prometheus.Gauge
}

// FeatureMetrics holds per-feature metrics
type FeatureMetrics struct {
	XDPStats  *ProgramMetrics
	TCStats   *ProgramMetrics
	SockStats *ProgramMetrics
}

// ProgramMetrics holds metrics for a specific program type
type ProgramMetrics struct {
	Packets prometheus.Counter
	Errors  prometheus.Counter
}

// NewMetrics creates a new Prometheus metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		PacketsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flywall_ebpf_packets_processed_total",
			Help: "Total number of packets processed by eBPF programs",
		}),
		PacketsDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flywall_ebpf_packets_dropped_total",
			Help: "Total number of packets dropped by eBPF programs",
		}),
		PacketsPassed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flywall_ebpf_packets_passed_total",
			Help: "Total number of packets passed by eBPF programs",
		}),
		BytesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flywall_ebpf_bytes_processed_total",
			Help: "Total number of bytes processed by eBPF programs",
		}),

		Features: &FeatureMetrics{
			XDPStats: &ProgramMetrics{
				Packets: prometheus.NewCounter(prometheus.CounterOpts{
					Name: "flywall_ebpf_xdp_packets_total",
					Help: "Total number of packets processed by XDP programs",
				}),
				Errors: prometheus.NewCounter(prometheus.CounterOpts{
					Name: "flywall_ebpf_xdp_errors_total",
					Help: "Total number of errors in XDP programs",
				}),
			},
			TCStats: &ProgramMetrics{
				Packets: prometheus.NewCounter(prometheus.CounterOpts{
					Name: "flywall_ebpf_tc_packets_total",
					Help: "Total number of packets processed by TC programs",
				}),
				Errors: prometheus.NewCounter(prometheus.CounterOpts{
					Name: "flywall_ebpf_tc_errors_total",
					Help: "Total number of errors in TC programs",
				}),
			},
			SockStats: &ProgramMetrics{
				Packets: prometheus.NewCounter(prometheus.CounterOpts{
					Name: "flywall_ebpf_socket_packets_total",
					Help: "Total number of packets processed by socket filters",
				}),
				Errors: prometheus.NewCounter(prometheus.CounterOpts{
					Name: "flywall_ebpf_socket_errors_total",
					Help: "Total number of errors in socket filters",
				}),
			},
		},

		MapEntries: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flywall_ebpf_map_entries",
			Help: "Number of entries in eBPF maps",
		}, []string{"map_name"}),

		MapUpdates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flywall_ebpf_map_updates_total",
			Help: "Total number of eBPF map updates",
		}, []string{"map_name", "operation"}),

		HookAttached: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flywall_ebpf_hook_attached",
			Help: "Whether an eBPF hook is attached (1 for attached, 0 for detached)",
		}, []string{"hook_type", "interface"}),

		HookErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flywall_ebpf_hook_errors_total",
			Help: "Total number of eBPF hook errors",
		}, []string{"hook_type", "error_type"}),

		DNSBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flywall_ebpf_dns_blocked_total",
			Help: "Total number of DNS queries blocked",
		}),

		DNSQueries: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flywall_ebpf_dns_queries_total",
			Help: "Total number of DNS queries processed",
		}),

		DNSDomainCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "flywall_ebpf_dns_blocklist_domains",
			Help: "Number of domains in the DNS blocklist",
		}),
	}
}

// Describe implements prometheus.Collector
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	// General metrics
	m.PacketsProcessed.Describe(ch)
	m.PacketsDropped.Describe(ch)
	m.PacketsPassed.Describe(ch)
	m.BytesProcessed.Describe(ch)

	// Feature metrics
	m.Features.XDPStats.Packets.Describe(ch)
	m.Features.XDPStats.Errors.Describe(ch)
	m.Features.TCStats.Packets.Describe(ch)
	m.Features.TCStats.Errors.Describe(ch)
	m.Features.SockStats.Packets.Describe(ch)
	m.Features.SockStats.Errors.Describe(ch)

	// Map metrics
	m.MapEntries.Describe(ch)
	m.MapUpdates.Describe(ch)

	// Hook metrics
	m.HookAttached.Describe(ch)
	m.HookErrors.Describe(ch)

	// DNS metrics
	m.DNSBlocked.Describe(ch)
	m.DNSQueries.Describe(ch)
	m.DNSDomainCount.Describe(ch)
}

// Collect implements prometheus.Collector
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	// General metrics
	m.PacketsProcessed.Collect(ch)
	m.PacketsDropped.Collect(ch)
	m.PacketsPassed.Collect(ch)
	m.BytesProcessed.Collect(ch)

	// Feature metrics
	m.Features.XDPStats.Packets.Collect(ch)
	m.Features.XDPStats.Errors.Collect(ch)
	m.Features.TCStats.Packets.Collect(ch)
	m.Features.TCStats.Errors.Collect(ch)
	m.Features.SockStats.Packets.Collect(ch)
	m.Features.SockStats.Errors.Collect(ch)

	// Map metrics
	m.MapEntries.Collect(ch)
	m.MapUpdates.Collect(ch)

	// Hook metrics
	m.HookAttached.Collect(ch)
	m.HookErrors.Collect(ch)

	// DNS metrics
	m.DNSBlocked.Collect(ch)
	m.DNSQueries.Collect(ch)
	m.DNSDomainCount.Collect(ch)
}

// RegisterMetrics registers all metrics with Prometheus
func (m *Metrics) RegisterMetrics() {
	prometheus.MustRegister(m)
}

// UpdateFromStats updates metrics from eBPF statistics
func (m *Metrics) UpdateFromStats(stats interface{}) {
	// This will be implemented to update metrics from the eBPF stats
	// For now, it's a placeholder
}
