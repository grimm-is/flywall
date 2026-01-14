package metrics

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/clock"

	"grimm.is/flywall/internal/logging"
)

// Collector gathers metrics from nftables and updates the Prometheus registry.
type Collector struct {
	registry *Registry
	logger   *logging.Logger
	interval time.Duration
	stopCh   chan struct{}

	// Cached metrics for API access
	mu             sync.RWMutex
	lastUpdate     time.Time
	interfaceStats map[string]*InterfaceStats
	policyStats    map[string]*PolicyStats
	serviceStats   *ServiceStats
	systemStats    *SystemStats
	conntrackStats *ConntrackStats

	// Baseline persistence for restart continuity
	baselineBucket BaselinePersister

	// Reload counters for testing
	reloadSuccess int64
	reloadFailure int64
}

// BaselinePersister interface for saving/loading counter baselines.
// This allows the collector to persist baselines without depending on state package directly.
type BaselinePersister interface {
	SaveInterfaceBaseline(name string, rxBytes, txBytes uint64) error
	LoadInterfaceBaseline(name string) (rxBytes, txBytes uint64, err error)
	SavePolicyBaseline(key string, packets, bytes uint64) error
	LoadPolicyBaseline(key string) (packets, bytes uint64, err error)
}

// InterfaceStats holds traffic statistics for a network interface.
type InterfaceStats struct {
	Name      string  `json:"name"`
	Zone      string  `json:"zone,omitempty"`
	RxBytes   uint64  `json:"rx_bytes"`
	TxBytes   uint64  `json:"tx_bytes"`
	RxPackets uint64  `json:"rx_packets"`
	TxPackets uint64  `json:"tx_packets"`
	RxErrors  uint64  `json:"rx_errors"`
	TxErrors  uint64  `json:"tx_errors"`
	RxDropped uint64  `json:"rx_dropped"`
	TxDropped uint64  `json:"tx_dropped"`
	RxBytesPS float64 `json:"rx_bytes_per_sec"`
	TxBytesPS float64 `json:"tx_bytes_per_sec"`
	LinkUp    bool    `json:"link_up"`
	Speed     uint64  `json:"speed_mbps,omitempty"`

	// Previous values for rate calculation (not exported to JSON)
	prevRxBytes   uint64    `json:"-"`
	prevTxBytes   uint64    `json:"-"`
	prevTimestamp time.Time `json:"-"`

	// Baseline offset for counter resets (persisted across restarts)
	baselineRxBytes uint64 `json:"-"`
	baselineTxBytes uint64 `json:"-"`
}

// PolicyStats holds firewall policy match statistics.
type PolicyStats struct {
	Name      string `json:"name"`
	FromZone  string `json:"from_zone"`
	ToZone    string `json:"to_zone"`
	Packets   uint64 `json:"packets"`
	Bytes     uint64 `json:"bytes"`
	Accepted  uint64 `json:"accepted"`
	Dropped   uint64 `json:"dropped"`
	Rejected  uint64 `json:"rejected"`
	LastMatch int64  `json:"last_match_unix,omitempty"`

	// Rate calculations (packets/sec, bytes/sec)
	PacketsPS float64 `json:"packets_per_sec"`
	BytesPS   float64 `json:"bytes_per_sec"`

	// Previous values for rate calculation (not exported to JSON)
	prevPackets   uint64    `json:"-"`
	prevBytes     uint64    `json:"-"`
	prevTimestamp time.Time `json:"-"`

	// Baseline offset for counter resets (persisted across restarts)
	baselinePackets uint64 `json:"-"`
	baselineBytes   uint64 `json:"-"`
}

// ServiceStats holds statistics for firewall services.
type ServiceStats struct {
	DHCP *DHCPStats `json:"dhcp"`
	DNS  *DNSStats  `json:"dns"`
}

// DHCPStats holds DHCP server statistics.
type DHCPStats struct {
	Enabled      bool   `json:"enabled"`
	ActiveLeases int    `json:"active_leases"`
	TotalLeases  int    `json:"total_leases"`
	Discovers    uint64 `json:"discovers"`
	Offers       uint64 `json:"offers"`
	Requests     uint64 `json:"requests"`
	Acks         uint64 `json:"acks"`
	Naks         uint64 `json:"naks"`
	Releases     uint64 `json:"releases"`
}

// DNSStats holds DNS server statistics.
type DNSStats struct {
	Enabled     bool   `json:"enabled"`
	Queries     uint64 `json:"queries"`
	CacheHits   uint64 `json:"cache_hits"`
	CacheMisses uint64 `json:"cache_misses"`
	Blocked     uint64 `json:"blocked"`
	Forwarded   uint64 `json:"forwarded"`
	CacheSize   int    `json:"cache_size"`
}

// SystemStats holds system-level statistics.
type SystemStats struct {
	Uptime        int64   `json:"uptime_seconds"`
	LoadAvg1      float64 `json:"load_avg_1"`
	LoadAvg5      float64 `json:"load_avg_5"`
	LoadAvg15     float64 `json:"load_avg_15"`
	MemTotal      uint64  `json:"mem_total_bytes"`
	MemFree       uint64  `json:"mem_free_bytes"`
	MemAvailable  uint64  `json:"mem_available_bytes"`
	CPUUser       float64 `json:"cpu_user_percent"`
	CPUSystem     float64 `json:"cpu_system_percent"`
	CPUIdle       float64 `json:"cpu_idle_percent"`
	KernelErrors  uint64  `json:"kernel_errors"`
	NetfilterDrop uint64  `json:"netfilter_drop"`
}

// ConntrackStats holds connection tracking statistics.
type ConntrackStats struct {
	Current      int    `json:"current"`
	Max          int    `json:"max"`
	Searched     uint64 `json:"searched"`
	Found        uint64 `json:"found"`
	New          uint64 `json:"new"`
	Invalid      uint64 `json:"invalid"`
	Ignore       uint64 `json:"ignore"`
	Delete       uint64 `json:"delete"`
	Insert       uint64 `json:"insert"`
	InsertFailed uint64 `json:"insert_failed"`
	Drop         uint64 `json:"drop"`
	EarlyDrop    uint64 `json:"early_drop"`
}

// NewCollector creates a new metrics collector.
func NewCollector(logger *logging.Logger, interval time.Duration) *Collector {
	return &Collector{
		registry:       Get(),
		logger:         logger,
		interval:       interval,
		stopCh:         make(chan struct{}),
		interfaceStats: make(map[string]*InterfaceStats),
		policyStats:    make(map[string]*PolicyStats),
		serviceStats:   &ServiceStats{DHCP: &DHCPStats{}, DNS: &DNSStats{}},
		systemStats:    &SystemStats{},
		conntrackStats: &ConntrackStats{},
	}
}

// SetBaselinePersister sets the baseline persister for saving/loading counter baselines.
// Call this before Start() to enable baseline persistence.
func (c *Collector) SetBaselinePersister(bp BaselinePersister) {
	c.baselineBucket = bp
}

// Start begins the metrics collection loop.
func (c *Collector) Start() {
	c.logger.Info("Starting metrics collector", "interval", c.interval.String())

	// Load baselines from state store
	c.loadBaselines()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collectMetrics()
		case <-c.stopCh:
			c.logger.Info("Stopping metrics collector")
			return
		}
	}
}

// Stop stops the metrics collection loop.
func (c *Collector) Stop() {
	// Save baselines before stopping
	c.saveBaselines()
	close(c.stopCh)
}

// loadBaselines loads counter baselines from state store on startup.
func (c *Collector) loadBaselines() {
	if c.baselineBucket == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Load interface baselines
	for name, stats := range c.interfaceStats {
		rxBytes, txBytes, err := c.baselineBucket.LoadInterfaceBaseline(name)
		if err == nil {
			stats.baselineRxBytes = rxBytes
			stats.baselineTxBytes = txBytes
			c.logger.Debug("Loaded interface baseline", "interface", name, "rx", rxBytes, "tx", txBytes)
		}
	}

	// Load policy baselines
	for key, stats := range c.policyStats {
		packets, bytes, err := c.baselineBucket.LoadPolicyBaseline(key)
		if err == nil {
			stats.baselinePackets = packets
			stats.baselineBytes = bytes
			c.logger.Debug("Loaded policy baseline", "policy", key, "packets", packets, "bytes", bytes)
		}
	}
}

// saveBaselines saves current counter values as baselines for next startup.
func (c *Collector) saveBaselines() {
	if c.baselineBucket == nil {
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Save interface baselines
	for name, stats := range c.interfaceStats {
		if err := c.baselineBucket.SaveInterfaceBaseline(name, stats.RxBytes, stats.TxBytes); err != nil {
			c.logger.Warn("Failed to save interface baseline", "interface", name, "error", err)
		}
	}

	// Save policy baselines
	for key, stats := range c.policyStats {
		if err := c.baselineBucket.SavePolicyBaseline(key, stats.Packets, stats.Bytes); err != nil {
			c.logger.Warn("Failed to save policy baseline", "policy", key, "error", err)
		}
	}

	c.logger.Info("Saved metrics baselines", "interfaces", len(c.interfaceStats), "policies", len(c.policyStats))
}

// collectMetrics gathers all metrics and updates the registry.
func (c *Collector) collectMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Collect interface statistics from /sys/class/net
	if err := c.collectInterfaceStatsFromSys(ctx); err != nil {
		c.logger.Warn("Failed to collect interface stats from sys", "error", err)
	}

	// Collect nftables counters
	if err := c.collectInterfaceStats(ctx); err != nil {
		c.logger.Warn("Failed to collect nftables interface stats", "error", err)
	}

	// Collect IPSet statistics
	if err := c.collectIPSetStats(ctx); err != nil {
		c.logger.Warn("Failed to collect IPSet stats", "error", err)
	}

	// Collect rule match counters
	if err := c.collectRuleStats(ctx); err != nil {
		c.logger.Warn("Failed to collect rule stats", "error", err)
	}

	// Collect policy chain counters
	if err := c.collectPolicyStats(ctx); err != nil {
		c.logger.Warn("Failed to collect policy stats", "error", err)
	}

	// Collect conntrack statistics
	if err := c.collectConntrackStats(ctx); err != nil {
		c.logger.Warn("Failed to collect conntrack stats", "error", err)
	}

	// Collect system statistics
	if err := c.collectSystemStats(ctx); err != nil {
		c.logger.Warn("Failed to collect system stats", "error", err)
	}

	c.lastUpdate = clock.Now()
}

// collectInterfaceStats gathers interface traffic counters from nftables.
func (c *Collector) collectInterfaceStats(_ context.Context) error {
	// Use native netlink-based collection instead of exec.Command
	ifaceStats, err := collectInterfaceStatsNative("flywall")
	if err != nil {
		return err
	}

	for ifaceName, counters := range ifaceStats {
		// Interface metrics need 2 labels: interface, zone
		// Zone is unknown from nftables counters, use empty string
		c.registry.InterfaceRxBytes.WithLabelValues(ifaceName, "").Set(float64(counters.RxBytes))
		c.registry.InterfaceRxPackets.WithLabelValues(ifaceName, "").Set(float64(counters.RxPackets))
		c.registry.InterfaceTxBytes.WithLabelValues(ifaceName, "").Set(float64(counters.TxBytes))
		c.registry.InterfaceTxPackets.WithLabelValues(ifaceName, "").Set(float64(counters.TxPackets))
	}

	return nil
}

// collectIPSetStats gathers IPSet size and update information.
func (c *Collector) collectIPSetStats(_ context.Context) error {
	// Use native netlink-based collection instead of exec.Command
	nftStats, err := collectNFTablesNative("flywall")
	if err != nil {
		return err
	}

	for setName, elementCount := range nftStats.Sets {
		// IPSetSize needs 2 labels: name, type
		// Use "ipv4_addr" as default type since we don't track set types yet
		c.registry.IPSetSize.WithLabelValues(setName, "ipv4_addr").Set(float64(elementCount))
	}

	return nil
}

// collectRuleStats gathers rule match counters from nftables.
func (c *Collector) collectRuleStats(_ context.Context) error {
	// Use native netlink-based collection instead of exec.Command
	nftStats, err := collectNFTablesNative("flywall")
	if err != nil {
		return err
	}

	// Process rule stats by chain:action
	for key, packets := range nftStats.RuleStats {
		// key format: "chain:action"
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		chain := parts[0]
		action := parts[1]

		c.registry.RuleMatches.WithLabelValues(chain, "unknown", action).Add(float64(packets))

		// Update action-specific counters
		if action == "drop" {
			c.registry.DroppedPackets.WithLabelValues(chain, "rule").Add(float64(packets))
		} else if action == "accept" {
			c.registry.AcceptedPackets.WithLabelValues(chain, "rule").Add(float64(packets))
		}
	}

	return nil
}

// UpdateSystemMetrics updates system-level metrics.
func (c *Collector) UpdateSystemMetrics(uptime time.Duration) {
	c.registry.Uptime.Set(uptime.Seconds())
}

// IncrementConfigReload increments the config reload counter.
func (c *Collector) IncrementConfigReload(success bool) {
	status := "success"
	if success {
		c.reloadSuccess++
	} else {
		status = "failure"
		c.reloadFailure++
	}
	c.registry.ConfigReload.WithLabelValues(status).Inc()
}

// GetReloadCounts returns the internal reload success/failure counts (for testing).
func (c *Collector) GetReloadCounts() (success, failure int64) {
	return c.reloadSuccess, c.reloadFailure
}

// collectInterfaceStatsFromSys gathers interface stats from /sys/class/net.
func (c *Collector) collectInterfaceStatsFromSys(_ context.Context) error {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return fmt.Errorf("failed to read /sys/class/net: %w", err)
	}

	now := clock.Now()

	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue // Skip loopback
		}

		stats, ok := c.interfaceStats[name]
		if !ok {
			stats = &InterfaceStats{Name: name}
			c.interfaceStats[name] = stats
		}

		basePath := fmt.Sprintf("/sys/class/net/%s/statistics", name)

		// Read current counters
		currentRxBytes := readSysUint64(basePath + "/rx_bytes")
		currentTxBytes := readSysUint64(basePath + "/tx_bytes")

		// Calculate rates with reset detection
		if !stats.prevTimestamp.IsZero() {
			elapsed := now.Sub(stats.prevTimestamp).Seconds()
			if elapsed > 0 {
				stats.RxBytesPS = c.calculateRate(currentRxBytes, stats.prevRxBytes, elapsed)
				stats.TxBytesPS = c.calculateRate(currentTxBytes, stats.prevTxBytes, elapsed)
			}
		}

		// Store for next iteration
		stats.prevRxBytes = currentRxBytes
		stats.prevTxBytes = currentTxBytes
		stats.prevTimestamp = now

		// Apply baseline offset for cumulative counters (handles restarts)
		stats.RxBytes = stats.baselineRxBytes + currentRxBytes
		stats.TxBytes = stats.baselineTxBytes + currentTxBytes
		stats.RxPackets = readSysUint64(basePath + "/rx_packets")
		stats.TxPackets = readSysUint64(basePath + "/tx_packets")
		stats.RxErrors = readSysUint64(basePath + "/rx_errors")
		stats.TxErrors = readSysUint64(basePath + "/tx_errors")
		stats.RxDropped = readSysUint64(basePath + "/rx_dropped")
		stats.TxDropped = readSysUint64(basePath + "/tx_dropped")

		// Check link state
		operstate, _ := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/operstate", name))
		stats.LinkUp = strings.TrimSpace(string(operstate)) == "up"

		// Get speed if available
		speedData, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/speed", name))
		if err == nil {
			speed, _ := strconv.ParseUint(strings.TrimSpace(string(speedData)), 10, 64)
			if speed > 0 && speed < 100000 { // Sanity check
				stats.Speed = speed
			}
		}

		// Update Prometheus metrics
		c.registry.InterfaceRxBytes.WithLabelValues(name, stats.Zone).Set(float64(stats.RxBytes))
		c.registry.InterfaceTxBytes.WithLabelValues(name, stats.Zone).Set(float64(stats.TxBytes))
		c.registry.InterfaceRxPackets.WithLabelValues(name, stats.Zone).Set(float64(stats.RxPackets))
		c.registry.InterfaceTxPackets.WithLabelValues(name, stats.Zone).Set(float64(stats.TxPackets))
		c.registry.InterfaceErrors.WithLabelValues(name, "rx").Set(float64(stats.RxErrors))
		c.registry.InterfaceErrors.WithLabelValues(name, "tx").Set(float64(stats.TxErrors))
	}

	return nil
}

// calculateRate computes the rate between two counter values, handling resets.
// If current < previous (counter reset), treats current as the delta from zero.
func (c *Collector) calculateRate(current, previous uint64, elapsedSeconds float64) float64 {
	if elapsedSeconds <= 0 {
		return 0
	}

	var delta uint64
	if current < previous {
		// Counter reset detected - use current value as delta
		// (assumes reset to zero, current is new accumulated value)
		delta = current
		c.logger.Debug("Counter reset detected", "current", current, "previous", previous)
	} else {
		delta = current - previous
	}

	return float64(delta) / elapsedSeconds
}

// collectPolicyStats gathers firewall policy chain counters.
func (c *Collector) collectPolicyStats(_ context.Context) error {
	// Use native netlink-based collection instead of exec.Command
	nftStats, err := collectNFTablesNative("flywall")
	if err != nil {
		return nil // Non-fatal
	}

	now := clock.Now()

	// Process policy counters
	for policyName, counters := range nftStats.PolicyStats {
		// Parse policy name: policy-wan-to-lan
		policyParts := strings.Split(policyName, "-")
		if len(policyParts) >= 4 {
			fromZone := policyParts[1]
			toZone := policyParts[3]
			key := fromZone + "->" + toZone

			stats, ok := c.policyStats[key]
			if !ok {
				stats = &PolicyStats{
					Name:     policyName,
					FromZone: fromZone,
					ToZone:   toZone,
				}
				c.policyStats[key] = stats
			}

			// Calculate rates with reset detection
			if !stats.prevTimestamp.IsZero() {
				elapsed := now.Sub(stats.prevTimestamp).Seconds()
				if elapsed > 0 {
					stats.PacketsPS = c.calculateRate(counters.Packets, stats.prevPackets, elapsed)
					stats.BytesPS = c.calculateRate(counters.Bytes, stats.prevBytes, elapsed)
				}
			}

			// Store for next iteration
			stats.prevPackets = counters.Packets
			stats.prevBytes = counters.Bytes
			stats.prevTimestamp = now

			// Apply baseline offset for cumulative counters
			stats.Packets = stats.baselinePackets + counters.Packets
			stats.Bytes = stats.baselineBytes + counters.Bytes
		}
	}

	return nil
}

// collectConntrackStats gathers connection tracking statistics.
func (c *Collector) collectConntrackStats(_ context.Context) error {
	// Read current conntrack count
	countData, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_count")
	if err == nil {
		count, _ := strconv.Atoi(strings.TrimSpace(string(countData)))
		c.conntrackStats.Current = count
		c.registry.ConntrackCount.Set(float64(count))
	}

	// Read max conntrack
	maxData, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_max")
	if err == nil {
		max, _ := strconv.Atoi(strings.TrimSpace(string(maxData)))
		c.conntrackStats.Max = max
		c.registry.ConntrackMax.Set(float64(max))
	}

	// Read conntrack stats from /proc/net/stat/nf_conntrack
	file, err := os.Open("/proc/net/stat/nf_conntrack")
	if err != nil {
		return nil // Non-fatal
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue // Skip header
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 17 {
			// Fields: entries searched found new invalid ignore delete delete_list insert insert_failed drop early_drop icmp_error expect_new expect_create expect_delete search_restart
			c.conntrackStats.Searched, _ = strconv.ParseUint(fields[1], 16, 64)
			c.conntrackStats.Found, _ = strconv.ParseUint(fields[2], 16, 64)
			c.conntrackStats.New, _ = strconv.ParseUint(fields[3], 16, 64)
			c.conntrackStats.Invalid, _ = strconv.ParseUint(fields[4], 16, 64)
			c.conntrackStats.Ignore, _ = strconv.ParseUint(fields[5], 16, 64)
			c.conntrackStats.Delete, _ = strconv.ParseUint(fields[6], 16, 64)
			c.conntrackStats.Insert, _ = strconv.ParseUint(fields[8], 16, 64)
			c.conntrackStats.InsertFailed, _ = strconv.ParseUint(fields[9], 16, 64)
			c.conntrackStats.Drop, _ = strconv.ParseUint(fields[10], 16, 64)
			c.conntrackStats.EarlyDrop, _ = strconv.ParseUint(fields[11], 16, 64)
			break // Only need first CPU's cumulative stats
		}
	}

	return nil
}

// collectSystemStats gathers system-level statistics.
func (c *Collector) collectSystemStats(_ context.Context) error {
	// Read uptime
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err == nil {
		fields := strings.Fields(string(uptimeData))
		if len(fields) >= 1 {
			uptime, _ := strconv.ParseFloat(fields[0], 64)
			c.systemStats.Uptime = int64(uptime)
		}
	}

	// Read load average
	loadData, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		fields := strings.Fields(string(loadData))
		if len(fields) >= 3 {
			c.systemStats.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
			c.systemStats.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
			c.systemStats.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)
		}
	}

	// Read memory info
	memFile, err := os.Open("/proc/meminfo")
	if err == nil {
		defer memFile.Close()
		scanner := bufio.NewScanner(memFile)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				value, _ := strconv.ParseUint(fields[1], 10, 64)
				value *= 1024 // Convert from KB to bytes
				switch {
				case strings.HasPrefix(line, "MemTotal:"):
					c.systemStats.MemTotal = value
				case strings.HasPrefix(line, "MemFree:"):
					c.systemStats.MemFree = value
				case strings.HasPrefix(line, "MemAvailable:"):
					c.systemStats.MemAvailable = value
				}
			}
		}
	}

	// Read kernel errors from dmesg (last count)
	// This is a simplified approach - real implementation might track delta
	cmd := exec.Command("dmesg", "--level=err,warn", "-c")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		var errorCount uint64
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				errorCount++
			}
		}
		c.systemStats.KernelErrors = errorCount
	}

	// Read netfilter drop stats
	nfDropData, err := os.ReadFile("/proc/net/netfilter/nf_log/0")
	if err == nil {
		// Parse netfilter log stats if available
		_ = nfDropData
	}

	return nil
}

// readSysUint64 reads a uint64 value from a sysfs file.
func readSysUint64(path string) uint64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	val, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return val
}

// GetInterfaceStats returns a copy of the current interface statistics.
func (c *Collector) GetInterfaceStats() map[string]*InterfaceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*InterfaceStats, len(c.interfaceStats))
	for k, v := range c.interfaceStats {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetPolicyStats returns a copy of the current policy statistics.
func (c *Collector) GetPolicyStats() map[string]*PolicyStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*PolicyStats, len(c.policyStats))
	for k, v := range c.policyStats {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetServiceStats returns a copy of the current service statistics.
func (c *Collector) GetServiceStats() *ServiceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &ServiceStats{
		DHCP: &DHCPStats{
			Enabled:      c.serviceStats.DHCP.Enabled,
			ActiveLeases: c.serviceStats.DHCP.ActiveLeases,
			TotalLeases:  c.serviceStats.DHCP.TotalLeases,
			Discovers:    c.serviceStats.DHCP.Discovers,
			Offers:       c.serviceStats.DHCP.Offers,
			Requests:     c.serviceStats.DHCP.Requests,
			Acks:         c.serviceStats.DHCP.Acks,
			Naks:         c.serviceStats.DHCP.Naks,
			Releases:     c.serviceStats.DHCP.Releases,
		},
		DNS: &DNSStats{
			Enabled:     c.serviceStats.DNS.Enabled,
			Queries:     c.serviceStats.DNS.Queries,
			CacheHits:   c.serviceStats.DNS.CacheHits,
			CacheMisses: c.serviceStats.DNS.CacheMisses,
			Blocked:     c.serviceStats.DNS.Blocked,
			Forwarded:   c.serviceStats.DNS.Forwarded,
			CacheSize:   c.serviceStats.DNS.CacheSize,
		},
	}
}

// GetSystemStats returns a copy of the current system statistics.
func (c *Collector) GetSystemStats() *SystemStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	copy := *c.systemStats
	return &copy
}

// GetConntrackStats returns a copy of the current conntrack statistics.
func (c *Collector) GetConntrackStats() *ConntrackStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	copy := *c.conntrackStats
	return &copy
}

// GetLastUpdate returns the timestamp of the last metrics collection.
func (c *Collector) GetLastUpdate() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdate
}

// SetInterfaceZone sets the zone for an interface (called from config).
func (c *Collector) SetInterfaceZone(iface, zone string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if stats, ok := c.interfaceStats[iface]; ok {
		stats.Zone = zone
	} else {
		c.interfaceStats[iface] = &InterfaceStats{Name: iface, Zone: zone}
	}
}

// UpdateDHCPStats updates DHCP service statistics.
func (c *Collector) UpdateDHCPStats(enabled bool, activeLeases, totalLeases int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.serviceStats.DHCP.Enabled = enabled
	c.serviceStats.DHCP.ActiveLeases = activeLeases
	c.serviceStats.DHCP.TotalLeases = totalLeases
}

// UpdateDNSStats updates DNS service statistics.
func (c *Collector) UpdateDNSStats(enabled bool, queries, cacheHits, cacheMisses, blocked uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.serviceStats.DNS.Enabled = enabled
	c.serviceStats.DNS.Queries = queries
	c.serviceStats.DNS.CacheHits = cacheHits
	c.serviceStats.DNS.CacheMisses = cacheMisses
	c.serviceStats.DNS.Blocked = blocked
}
