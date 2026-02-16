// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dns

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/install"

	"grimm.is/flywall/internal/clock"

	"context"

	"path/filepath"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/services"
	"grimm.is/flywall/internal/services/dns/querylog"
	"grimm.is/flywall/internal/upgrade"

	"github.com/miekg/dns"
)

// Service implements a recursive DNS resolver.
// It supports:
// - Upstream forwarding (UDP, DoT, DoH)
// - Local records (A, AAAA, PTR, CNAME)
// - Blocklists (Ads/Malware)
// - Caching (RFC compliant)
// - Firewall Integration ("DNS Wall"): Authorizes IPs in the firewall upon successful resolution.
type Service struct {
	servers          []*dns.Server
	config           *config.DNSServer
	upstreams        []upstream                  // Unified list of upstreams (UDP, DoT, DoH)
	dynamicUpstreams []upstream                  // From DHCP/etc
	records          map[string]config.DNSRecord // FQDN -> Record
	blockedDomains   map[string]bool             // Blocked domains

	// Sharded Cache
	shards [256]*cacheShard

	mu          sync.RWMutex
	running     bool
	stopCleanup chan struct{}
	upgradeMgr  *upgrade.Manager
	fw          ValidatingFirewall

	// Egress Filter State
	egressFilterEnabled bool
	egressFilterTTL     int

	// Query Logging
	queryLog *querylog.Store
}

// ValidatingFirewall defines the interface for firewall authorization
type ValidatingFirewall interface {
	AuthorizeIP(ip net.IP, ttl time.Duration) error
} // ValidatingFirewall

type upstream struct {
	Addr       string
	Protocol   string // "udp", "tcp", "tcp-tls", "https"
	ServerName string // For TLS/HTTPS verification
	URL        string // For DoH (full URL)
}

type cachedResponse struct {
	msg       *dns.Msg
	expiresAt time.Time
}

type cacheShard struct {
	mu    sync.RWMutex
	items map[string]cachedResponse
}

func NewService(cfg *config.Config, logger *logging.Logger) *Service {
	s := &Service{
		config:         &config.DNSServer{},
		records:        make(map[string]config.DNSRecord),
		blockedDomains: make(map[string]bool),
		stopCleanup:    make(chan struct{}),
	}

	// Initialize cache shards
	for i := 0; i < 256; i++ {
		s.shards[i] = &cacheShard{
			items: make(map[string]cachedResponse),
		}
	}

	return s
}

// getShard returns the cache shard for a given key
func (s *Service) getShard(key string) *cacheShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return s.shards[h.Sum32()%256]
}

// SetFirewall sets the firewall manager for validation
func (s *Service) SetFirewall(fw ValidatingFirewall) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fw = fw
}

// SetUpgradeManager sets the upgrade manager for socket handoff.
func (s *Service) SetUpgradeManager(mgr *upgrade.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upgradeMgr = mgr
}

// SetQueryLog sets the query log store
func (s *Service) SetQueryLog(store *querylog.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queryLog = store
}

func (s *Service) Name() string {
	return "DNS"
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if len(s.servers) == 0 {
		// Nothing to start
		return nil
	}

	logging.Debug("[DNS] Starting %d servers...", len(s.servers))
	for i, srv := range s.servers {
		go func(srv *dns.Server, index int) {
			// user ActivateAndServe if generic listener/packetconn is set
			if srv.Listener != nil || srv.PacketConn != nil {
				if err := srv.ActivateAndServe(); err != nil {
					logging.Error("[DNS] Server %d error: %v", index, err)
				}
			} else {
				if err := srv.ListenAndServe(); err != nil {
					logging.Error("[DNS] Server %d error: %v", index, err)
				}
			}
		}(srv, i+1)
	}

	// Start cleanup manually since we manage it
	go s.startCacheCleanup()

	s.running = true
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	logging.Debug("[DNS] Stopping %d servers...", len(s.servers))
	for i, srv := range s.servers {
		if err := srv.Shutdown(); err != nil {
			logging.Error("[DNS] Failed to stop server %d: %v", i+1, err)
		}
	}

	// Stop cleanup
	select {
	case s.stopCleanup <- struct{}{}:
	default:
	}

	s.running = false
	return nil
}

// Reload reconfigures the service with minimal downtime.
func (s *Service) Reload(cfg *config.Config) (bool, error) {
	s.mu.RLock()
	wasRunning := s.running
	oldConfig := s.config
	s.mu.RUnlock()

	// Detect config mode
	activeMode := "legacy"
	if cfg.DNS != nil && (len(cfg.DNS.Serve) > 0 || len(cfg.DNS.Forwarders) > 0 || cfg.DNS.Mode != "") {
		activeMode = "new"
	}

	// Update Egress Filter State
	s.mu.Lock()
	if cfg.DNS != nil {
		s.egressFilterEnabled = cfg.DNS.EgressFilter
		s.egressFilterTTL = cfg.DNS.EgressFilterTTL
	} else {
		s.egressFilterEnabled = false
		s.egressFilterTTL = 0
	}
	s.mu.Unlock()

	// -------------------------------------------------------------
	// NEW CONFIG MODE (dns {})
	// -------------------------------------------------------------
	if activeMode == "new" {
		state := s.buildServerState(cfg)

		if wasRunning {
			logging.Info("[DNS] Restarting DNS server (New Config Mode)")
			s.Stop(context.Background())
		}

		newServers := s.buildServers(state.listenAddrs)

		s.mu.Lock()
		// Map back to legacy config structure for internal compatibility
		s.config = &config.DNSServer{
			ConditionalForwarders: cfg.DNS.ConditionalForwarders,
		}
		// Convert new mode forwarders (strings) to upstreams (udp)
		var newUpstreams []upstream
		for _, fwd := range state.forwarders {
			newUpstreams = append(newUpstreams, upstream{Addr: fwd, Protocol: "udp"})
		}
		s.upstreams = newUpstreams
		s.records = state.records
		s.blockedDomains = state.blockedDomains
		s.servers = newServers
		s.mu.Unlock()

		return true, s.Start(context.Background())
	}

	// -------------------------------------------------------------
	// LEGACY CONFIG MODE (dns_server {})
	// -------------------------------------------------------------
	dnsCfg := cfg.DNSServer
	if dnsCfg == nil || !dnsCfg.Enabled {
		if wasRunning {
			return true, s.Stop(context.Background())
		}
		return true, nil
	}

	// Check for external mode
	// ... (rest of legacy logic)

	// Return to legacy flow for diff minimality, or duplicate?
	// I'll rewrite the legacy part below or reuse existing via return.
	// Since I replaced the WHOLE function, I must provide legacy implementation here.

	// Copy of original legacy implementation:
	if dnsCfg.Mode == "external" {
		logging.Info("[DNS] External DNS server configured. Skipping built-in server startup.")
		if wasRunning {
			return true, s.Stop(context.Background())
		}
		return true, nil
	}

	// 1. Pre-load everything into local variables
	newRecords := make(map[string]config.DNSRecord)
	newBlocked := make(map[string]bool)
	var newServers []*dns.Server

	// Build upstreams (Legacy UDP + DoT + DoH)
	newUpstreams := s.buildStaticUpstreams(dnsCfg)

	// Build records map
	for _, zone := range dnsCfg.Zones {
		zoneName := dns.Fqdn(zone.Name)
		for _, rec := range zone.Records {
			fqdn := zoneName
			if rec.Name != "@" {
				fqdn = dns.Fqdn(rec.Name + "." + zoneName)
			}
			newRecords[strings.ToLower(fqdn)] = rec
		}
	}

	// Build static hosts map
	for _, host := range dnsCfg.Hosts {
		for _, hostname := range host.Hostnames {
			fqdn := dns.Fqdn(hostname)
			recType := "A"
			if ip := net.ParseIP(host.IP); ip != nil && ip.To4() == nil {
				recType = "AAAA"
			}
			rec := config.DNSRecord{
				Name:  hostname,
				Type:  recType,
				Value: host.IP,
				TTL:   3600,
			}
			newRecords[strings.ToLower(fqdn)] = rec
		}
	}

	// Load blocklists
	var err error
	newBlocked, err = loadBlocklistsFromConfig(dnsCfg.Blocklists)
	if err != nil {
		logging.Warn("[DNS] Warning: errors occurred loading blocklists: %v", err)
	}

	// Load /etc/hosts
	if f, err := os.Open("/etc/hosts"); err == nil {
		scanner := bufio.NewScanner(f)
		count := 0
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if idx := strings.Index(line, "#"); idx != -1 {
				line = strings.TrimSpace(line[:idx])
			}
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			ipStr := parts[0]
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			recType := "A"
			if ip.To4() == nil {
				recType = "AAAA"
			}
			for _, hostname := range parts[1:] {
				fqdn := dns.Fqdn(hostname)
				lowerName := strings.ToLower(fqdn)
				if _, exists := newRecords[lowerName]; !exists {
					newRecords[lowerName] = config.DNSRecord{
						Name:  hostname,
						Type:  recType,
						Value: ipStr,
						TTL:   3600,
					}
					count++
				}
			}
		}
		f.Close()
		logging.Info("[DNS] Loaded %d records from /etc/hosts", count)
	}

	// 2. Check if restart is needed
	listenersChanged := true
	if wasRunning && oldConfig != nil {
		oldL := oldConfig.ListenOn
		newL := dnsCfg.ListenOn
		if len(oldL) == 0 {
			oldL = []string{"0.0.0.0"}
		}
		if len(newL) == 0 {
			newL = []string{"0.0.0.0"}
		}

		if len(oldL) == len(newL) {
			match := true
			for i, v := range oldL {
				if v != newL[i] {
					match = false
					break
				}
			}
			if match {
				listenersChanged = false
			}
		}
	}

	// 3. Apply changes (Legacy)
	if listenersChanged {
		if wasRunning {
			logging.Info("[DNS] Restarting DNS server (listener config changed)")
			s.Stop(context.Background())
		}

		listeners := dnsCfg.ListenOn
		if len(listeners) == 0 {
			listeners = []string{"0.0.0.0"}
		}
		newServers = s.buildServers(listeners)

		s.mu.Lock()
		s.config = dnsCfg
		s.upstreams = newUpstreams
		s.records = newRecords
		s.blockedDomains = newBlocked
		s.servers = newServers
		s.mu.Unlock()

		return true, s.Start(context.Background())
	} else {
		// Hot Swap
		s.mu.Lock()
		s.config = dnsCfg
		s.upstreams = newUpstreams
		s.records = newRecords
		s.blockedDomains = newBlocked
		s.mu.Unlock()
		logging.Info("[DNS] Hot-reloaded configuration (no restart)")
		return true, nil
	}
}

func uniqueStrings(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)
	for _, val := range input {
		if !m[val] {
			m[val] = true
			u = append(u, val)
		}
	}
	return u
}

func (s *Service) buildServers(listeners []string) []*dns.Server {
	var newServers []*dns.Server
	for _, addr := range listeners {
		// UDP
		udpName := fmt.Sprintf("dns-udp-%s", addr)
		var pc net.PacketConn

		if s.upgradeMgr != nil {
			if existing, ok := s.upgradeMgr.GetPacketConn(udpName); ok {
				pc = existing
				logging.Info("[DNS] Inherited UDP socket %s", udpName)
			}
		}

		if pc == nil {
			var err error
			pc, err = net.ListenPacket("udp", net.JoinHostPort(addr, "53"))
			if err != nil {
				logging.Error("[DNS] Failed to bind UDP %s: %v", addr, err)
			} else if s.upgradeMgr != nil {
				s.upgradeMgr.RegisterPacketConn(udpName, pc)
			}
		}

		if pc != nil {
			newServers = append(newServers, &dns.Server{PacketConn: pc, Addr: pc.LocalAddr().String(), Net: "udp", Handler: s})
		}

		// TCP
		tcpName := fmt.Sprintf("dns-tcp-%s", addr)
		var list net.Listener

		if s.upgradeMgr != nil {
			if existing, ok := s.upgradeMgr.GetListener(tcpName); ok {
				list = existing
				logging.Info("[DNS] Inherited TCP listener %s", tcpName)
			}
		}

		if list == nil {
			var err error
			list, err = net.Listen("tcp", net.JoinHostPort(addr, "53"))
			if err != nil {
				logging.Error("[DNS] Failed to bind TCP %s: %v", addr, err)
			} else if s.upgradeMgr != nil {
				s.upgradeMgr.RegisterListener(tcpName, list)
			}
		}

		if list != nil {
			newServers = append(newServers, &dns.Server{Listener: list, Addr: list.Addr().String(), Net: "tcp", Handler: s})
		}
	}
	return newServers
}

func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Service) Status() services.ServiceStatus {
	return services.ServiceStatus{
		Name:    s.Name(),
		Running: s.IsRunning(),
	}
}

func (s *Service) loadHostsFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Warn("[DNS] Failed to open hosts file %s: %v", path, err)
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		ipStr := parts[0]
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}

		recType := "A"
		if ip.To4() == nil {
			recType = "AAAA"
		}

		for _, hostname := range parts[1:] {
			fqdn := dns.Fqdn(hostname)
			lowerName := strings.ToLower(fqdn)

			// Don't overwrite existing records (config takes precedence)
			if _, exists := s.records[lowerName]; !exists {
				s.records[lowerName] = config.DNSRecord{
					Name:  hostname,
					Type:  recType,
					Value: ipStr,
					TTL:   3600,
				}
				count++
			}
		}
	}
	logging.Info("[DNS] Loaded %d records from %s", count, path)
}

func loadBlocklistsFromConfig(blocklists []config.DNSBlocklist) (map[string]bool, error) {
	blocked := make(map[string]bool)
	cachePath := filepath.Join(install.GetCacheDir(), "blocklist_cache")

	for _, bl := range blocklists {
		if !bl.Enabled {
			continue
		}

		var domains []string
		var err error
		var source string

		if bl.URL != "" {
			// URL-based blocklist - download with cache fallback
			domains, err = DownloadBlocklistWithCache(bl.URL, cachePath)
			if err != nil {
				logging.Warn("[DNS] Failed to load blocklist %s from URL: %v", bl.Name, err)
				continue
			}
			source = bl.URL
		} else if bl.File != "" {
			// File-based blocklist
			f, err := os.Open(bl.File)
			if err != nil {
				logging.Warn("[DNS] Failed to open blocklist file %s: %v", bl.File, err)
				continue
			}

			// Use the existing parseBlocklist helper from blocklist.go if possible?
			// parseBlocklist takes io.Reader. Yes.
			domains, err = parseBlocklist(f)
			f.Close()

			if err != nil {
				logging.Warn("[DNS] Failed to parse blocklist file %s: %v", bl.File, err)
				continue
			}
			source = bl.File
		} else {
			logging.Warn("[DNS] Blocklist %s has no URL or file specified", bl.Name)
			continue
		}

		// Add domains to blocked set
		count := 0
		for _, d := range domains {
			domain := strings.ToLower(dns.Fqdn(d))
			blocked[domain] = true
			count++
		}
		logging.Info("[DNS] Loaded %d domains from blocklist %s (%s)", count, bl.Name, source)
	}

	return blocked, nil
}

// AddRecord adds or updates a dynamic DNS record.
func (s *Service) AddRecord(name string, ip net.IP) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fqdn := dns.Fqdn(name)

	rec := config.DNSRecord{
		Name:  name,
		Type:  "A",
		Value: ip.String(),
		TTL:   300,
	}
	if ip.To4() == nil {
		rec.Type = "AAAA"
	}

	s.records[strings.ToLower(fqdn)] = rec

	// Automatic PTR record
	ptrZone, err := dns.ReverseAddr(ip.String())
	if err == nil {
		s.records[strings.ToLower(ptrZone)] = config.DNSRecord{
			Name:  ptrZone,
			Type:  "PTR",
			Value: fqdn,
			TTL:   300,
		}
		logging.Debug("[DNS] Added dynamic PTR record: %s -> %s", ptrZone, fqdn)
	}

	logging.Debug("[DNS] Added dynamic record: %s -> %s", fqdn, ip)
}

// RemoveRecord removes a dynamic DNS record.
func (s *Service) RemoveRecord(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fqdn := dns.Fqdn(name)
	lowerName := strings.ToLower(fqdn)

	// If it's an A record, try to find and remove associated PTR
	if rec, ok := s.records[lowerName]; ok && (rec.Type == "A" || rec.Type == "AAAA") {
		ptrZone, err := dns.ReverseAddr(rec.Value)
		if err == nil {
			delete(s.records, strings.ToLower(ptrZone))
			logging.Debug("[DNS] Removed dynamic PTR record: %s", ptrZone)
		}
	}

	delete(s.records, lowerName)
	logging.Debug("[DNS] Removed dynamic record: %s", fqdn)
}

// UpdateBlockedDomains updates the set of blocked domains dynamically
func (s *Service) UpdateBlockedDomains(domains []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, d := range domains {
		domain := strings.ToLower(dns.Fqdn(d))
		if !s.blockedDomains[domain] {
			s.blockedDomains[domain] = true
			count++
		}
	}
	if count > 0 {
		logging.Info("[DNS] Added %d domains to blocklist from threat intel", count)
	}
}

// UpdateForwarders updates the upstream DNS forwarders dynamically (e.g. from DHCP)
// UpdateForwarders updates the upstream DNS forwarders dynamically (e.g. from DHCP)
func (s *Service) UpdateForwarders(forwarders []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedup and validate
	var valid []upstream
	seen := make(map[string]bool)

	for _, fwd := range forwarders {
		if seen[fwd] {
			continue
		}
		// Basic IP check
		if net.ParseIP(fwd) != nil {
			valid = append(valid, upstream{
				Addr:     fwd,
				Protocol: "udp",
			})
			seen[fwd] = true
		}
	}

	s.dynamicUpstreams = valid
	logging.Debug("[DNS] Updated dynamic forwarders: %v", s.dynamicUpstreams)
}

// SyncFirewall re-authorizes all currently cached IPs in the firewall.
// This ensures that after a firewall reload, valid connections are not dropped.
func (s *Service) SyncFirewall() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.fw == nil {
		return
	}

	count := 0
	now := time.Now()

	for _, shard := range s.shards {
		shard.mu.RLock()
		for _, item := range shard.items {
			// Only sync valid items
			if now.After(item.expiresAt) {
				continue
			}

			// Extract answers
			for _, ans := range item.msg.Answer {
				if a, ok := ans.(*dns.A); ok {
					ttl := time.Until(item.expiresAt)
					if ttl > 0 {
						s.fw.AuthorizeIP(a.A, ttl)
						count++
					}
				} else if aaaa, ok := ans.(*dns.AAAA); ok {
					ttl := time.Until(item.expiresAt)
					if ttl > 0 {
						s.fw.AuthorizeIP(aaaa.AAAA, ttl)
						count++
					}
				}
			}
		}
		shard.mu.RUnlock()
	}

	logging.Debug("[DNS] SyncFirewall invoked (cache_count=%d)", count)
}

// RemoveBlockedDomains removes domains from the blocklist
func (s *Service) RemoveBlockedDomains(domains []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, d := range domains {
		domain := strings.ToLower(dns.Fqdn(d))
		delete(s.blockedDomains, domain)
	}
}

// Old Start/Stop removed
// Old Start/Stop removed

// ServeDNS handles incoming DNS requests.
// Pipeline:
// 1. Check Blocklists (if blocked -> NXDOMAIN)
// 2. Check Cache (if hit -> return cached response)
// 3. Check Local Records (hosts file, config records)
// 4. Conditional Forwarding (split-horizon)
// 5. Upstream Forwarding (UDP/DoT/DoH)
// 6. Fallback (NXDOMAIN)
func (s *Service) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	startTime := time.Now()
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())

	var (
		rcode        = dns.RcodeSuccess
		upstreamAddr string
		blocked      bool
		blockList    string
		resp         *dns.Msg
		pType        string
	)

	if len(r.Question) > 0 {
		pType = dns.TypeToString[r.Question[0].Qtype]
	}

	// Defer logging
	defer func() {
		if s.queryLog != nil && len(r.Question) > 0 {
			entry := querylog.Entry{
				Timestamp:  startTime,
				ClientIP:   clientIP,
				Domain:     strings.ToLower(r.Question[0].Name),
				Type:       pType,
				RCode:      dns.RcodeToString[rcode],
				Upstream:   upstreamAddr,
				DurationMs: time.Since(startTime).Milliseconds(),
				Blocked:    blocked,
				BlockList:  blockList,
			}
			// Async log
			go func() {
				if err := s.queryLog.RecordEntry(entry); err != nil {
					// Silent failure or debug log
				}
			}()
		}
	}()

	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Compress = false

	if len(r.Question) == 0 {
		w.WriteMsg(msg)
		return
	}

	q := r.Question[0]
	name := strings.ToLower(q.Name)

	// Check Blocklists
	s.mu.RLock()
	fmt.Fprintf(os.Stderr, "DEBUG: Check blocked\n")
	isBlocked := s.blockedDomains[name]
	// Also check without trailing dot
	if !isBlocked && strings.HasSuffix(name, ".") {
		isBlocked = s.blockedDomains[name[:len(name)-1]]
	}
	s.mu.RUnlock()

	if isBlocked {
		blocked = true
		logging.Debug("[DNS] Blocked query for %s", name)
		msg.Rcode = dns.RcodeNameError // NXDOMAIN
		rcode = dns.RcodeNameError
		w.WriteMsg(msg)
		return
	}

	// Check Cache
	cacheKey := fmt.Sprintf("%s:%d", name, q.Qtype)
	shard := s.getShard(cacheKey)

	shard.mu.RLock()
	fmt.Fprintf(os.Stderr, "DEBUG: Check cache\n")
	cached, found := shard.items[cacheKey]
	shard.mu.RUnlock()

	if found && clock.Now().Before(cached.expiresAt) {
		fmt.Fprintf(os.Stderr, "DEBUG: Cache hit\n")
		resp = cached.msg.Copy()
		resp.SetReply(r)
		rcode = resp.Rcode
		w.WriteMsg(resp)
		return
	}
	fmt.Fprintf(os.Stderr, "DEBUG: Cache miss or expired\n")

	// Local Lookup
	s.mu.RLock()
	fmt.Fprintf(os.Stderr, "DEBUG: Check local\n")
	rec, ok := s.records[name]
	s.mu.RUnlock()

	if ok {
		rr := s.createRR(q, rec)
		if rr != nil {
			msg.Answer = append(msg.Answer, rr)
			w.WriteMsg(msg)
			return
		}
	}

	// Conditional Forwarding
	fmt.Fprintf(os.Stderr, "DEBUG: Check conditional\n")
	for _, cf := range s.config.ConditionalForwarders {
		domain := dns.Fqdn(cf.Domain)
		if strings.HasSuffix(name, strings.ToLower(domain)) {
			// Convert string servers to upstreams (assume UDP for now)
			var cfUpstreams []upstream
			for _, srv := range cf.Servers {
				cfUpstreams = append(cfUpstreams, upstream{Addr: srv, Protocol: "udp"})
			}

			resp, upstreamAddr = s.forward(r, cfUpstreams)
			if resp != nil {
				rcode = resp.Rcode
				w.WriteMsg(resp)
			} else {
				rcode = dns.RcodeServerFailure
				dns.HandleFailed(w, r)
			}
			return
		}
	}

	// Forwarding
	// Merge static and dynamic upstreams
	var allUpstreams []upstream
	s.mu.RLock()
	fmt.Fprintf(os.Stderr, "DEBUG: Check forwarding\n")
	if len(s.upstreams) > 0 {
		allUpstreams = append(allUpstreams, s.upstreams...)
	}
	if len(s.dynamicUpstreams) > 0 {
		allUpstreams = append(allUpstreams, s.dynamicUpstreams...)
	}
	s.mu.RUnlock()

	if len(allUpstreams) > 0 {
		fmt.Fprintf(os.Stderr, "DEBUG: Forwarding to upstreams\n")
		resp, upstreamAddr = s.forward(r, allUpstreams)
		if resp != nil {
			rcode = resp.Rcode
			w.WriteMsg(resp)
		} else {
			rcode = dns.RcodeServerFailure
			dns.HandleFailed(w, r)
		}
		return
	}

	// NXDOMAIN
	fmt.Fprintf(os.Stderr, "DEBUG: NXDOMAIN\n")
	msg.Rcode = dns.RcodeNameError
	rcode = dns.RcodeNameError
	w.WriteMsg(msg)
}

func (s *Service) createRR(q dns.Question, rec config.DNSRecord) dns.RR {
	ttl := uint32(rec.TTL)
	if ttl == 0 {
		ttl = 3600
	}

	header := dns.RR_Header{
		Name:   q.Name,
		Rrtype: q.Qtype,
		Class:  dns.ClassINET,
		Ttl:    ttl,
	}

	if dns.TypeToString[q.Qtype] == rec.Type {
		switch q.Qtype {
		case dns.TypeA:
			if ip := net.ParseIP(rec.Value); ip != nil && ip.To4() != nil {
				return &dns.A{Hdr: header, A: ip.To4()}
			}
		case dns.TypeAAAA:
			if ip := net.ParseIP(rec.Value); ip != nil && ip.To16() != nil {
				return &dns.AAAA{Hdr: header, AAAA: ip.To16()}
			}
		case dns.TypeCNAME:
			return &dns.CNAME{Hdr: header, Target: dns.Fqdn(rec.Value)}
		case dns.TypeTXT:
			return &dns.TXT{Hdr: header, Txt: []string{rec.Value}}
		case dns.TypePTR:
			return &dns.PTR{Hdr: header, Ptr: dns.Fqdn(rec.Value)}
		case dns.TypeMX:
			var pref uint16
			var target string
			if n, _ := fmt.Sscanf(rec.Value, "%d %s", &pref, &target); n == 2 {
				return &dns.MX{Hdr: header, Preference: pref, Mx: dns.Fqdn(target)}
			}
		case dns.TypeSRV:
			var priority, weight, port uint16
			var target string
			if n, _ := fmt.Sscanf(rec.Value, "%d %d %d %s", &priority, &weight, &port, &target); n == 4 {
				return &dns.SRV{Hdr: header, Priority: priority, Weight: weight, Port: port, Target: dns.Fqdn(target)}
			}
		}
	}
	return nil
}

func (s *Service) forward(r *dns.Msg, upstreams []upstream) (*dns.Msg, string) {
	if r == nil || len(r.Question) == 0 {
		return nil, ""
	}

	c := new(dns.Client)
	c.Timeout = 2 * time.Second

	for _, up := range upstreams {
		var resp *dns.Msg
		var err error

		// Enable DNSSEC if configured
		if s.config.DNSSEC {
			setDO(r)
		}

		switch up.Protocol {
		case "tcp-tls": // DNS-over-TLS
			c.Net = "tcp-tls"
			c.TLSConfig = &tls.Config{
				ServerName: up.ServerName,
				MinVersion: tls.VersionTLS12,
			}
			addr := up.Addr
			if !strings.Contains(addr, ":") {
				addr = addr + ":853"
			}
			resp, _, err = c.Exchange(r, addr)

		case "https": // DNS-over-HTTPS
			c.Net = "https"
			// For DoH, Address is the URL
			resp, _, err = c.Exchange(r, up.URL)

		case "tcp":
			c.Net = "tcp"
			addr := up.Addr
			if !strings.Contains(addr, ":") {
				addr = addr + ":53"
			}
			resp, _, err = c.Exchange(r, addr)

		default: // udp
			c.Net = "udp"
			addr := up.Addr
			if !strings.Contains(addr, ":") {
				addr = addr + ":53"
			}
			resp, _, err = c.Exchange(r, addr)
		}

		if err == nil && resp != nil {
			// DNSSEC Validation Check
			if s.config.DNSSEC && !validateResponse(resp) {
				// Only warn for now, don't drop (Soft Fail)
				log.Printf("[DNSSEC] Warning: Response from %s not authenticated (AD bit unset)", up.Addr)
			}

			s.cacheResponse(r, resp)
			return resp, up.Addr
		}
	}

	// Failed to forward
	return nil, ""
}

// GetCache returns the current DNS cache entries for upgrade state preservation
func (s *Service) GetCache() []upgrade.DNSCacheEntry {
	var entries []upgrade.DNSCacheEntry

	for _, shard := range s.shards {
		shard.mu.RLock()
		for _, item := range shard.items {
			msgBytes, err := item.msg.Pack()
			if err != nil {
				log.Printf("Failed to pack DNS message for cache export: %v", err)
				continue
			}

			if len(item.msg.Question) > 0 {
				entries = append(entries, upgrade.DNSCacheEntry{
					Name:    item.msg.Question[0].Name,
					Type:    item.msg.Question[0].Qtype,
					Data:    msgBytes,
					Expires: item.expiresAt,
				})
			}
		}
		shard.mu.RUnlock()
	}
	return entries
}

func (s *Service) startCacheCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupCache()
		case <-s.stopCleanup:
			return
		}
	}
}

func (s *Service) cleanupCache() {
	now := clock.Now()
	cleaned := 0

	for _, shard := range s.shards {
		shard.mu.Lock()
		for key, entry := range shard.items {
			if now.After(entry.expiresAt) {
				delete(shard.items, key)
				cleaned++
			}
		}
		shard.mu.Unlock()
	}

	if cleaned > 0 {
		log.Printf("[DNS] Cleaned %d expired cache entries", cleaned)
	}
}

func (s *Service) cacheResponse(req, resp *dns.Msg) {
	if resp.Rcode != dns.RcodeSuccess || len(resp.Answer) == 0 {
		return
	}

	minTTL := uint32(3600) // Default max
	for _, rr := range resp.Answer {
		if rr.Header().Ttl < minTTL {
			minTTL = rr.Header().Ttl
		}
	}

	// Don't cache very short TTLs
	if minTTL > 5 {
		cacheKey := fmt.Sprintf("%s:%d", strings.ToLower(req.Question[0].Name), req.Question[0].Qtype)
		shard := s.getShard(cacheKey)

		shard.mu.Lock()
		// DOS Protection: Limit cache size with random eviction
		if len(shard.items) >= 1000 { // 1000 * 256 = 256k entries total capacity
			// Evict a random entry
			for k := range shard.items {
				delete(shard.items, k)
				break
			}
		}

		shard.items[cacheKey] = cachedResponse{
			msg:       resp,
			expiresAt: clock.Now().Add(time.Duration(minTTL) * time.Second),
		}
		shard.mu.Unlock()
	}

	// Snoop response for firewall authorization
	s.snoopResponse(resp)
}

// snoopResponse implements the "DNS Wall" (Egress Filtering) logic.
// It inspects valid DNS responses and extracts the resolved IP addresses.
// These IPs are then sent to the firewall manager to be added to a dynamic
// allowlist (ipset). This allows LAN clients to access only domains they
// have resolved, preventing direct IP access to malware C2 or unauthorized sites.
func (s *Service) snoopResponse(resp *dns.Msg) {
	s.mu.RLock()
	enabled := s.egressFilterEnabled
	customTTL := s.egressFilterTTL
	s.mu.RUnlock()

	if !enabled || s.fw == nil {
		return
	}

	for _, rr := range resp.Answer {
		var ip net.IP
		ttl := time.Duration(rr.Header().Ttl) * time.Second
		if customTTL > 0 {
			ttl = time.Duration(customTTL) * time.Second
		}

		switch v := rr.(type) {
		case *dns.A:
			ip = v.A
		case *dns.AAAA:
			ip = v.AAAA
		}

		if ip != nil {
			// Async authorization
			go func(ip net.IP, ttl time.Duration) {
				if err := s.fw.AuthorizeIP(ip, ttl); err != nil {
					// Low-level debugging only
					// s.logger.Debug("Failed to authorize IP", "ip", ip, "error", err)
				}
			}(ip, ttl)
		}
	}
}

func (s *Service) buildStaticUpstreams(cfg *config.DNSServer) []upstream {
	var upstreams []upstream

	// DoT
	for _, dot := range cfg.UpstreamDoT {
		if dot.Enabled {
			upstreams = append(upstreams, upstream{
				Addr:       dot.Server,
				Protocol:   "tcp-tls",
				ServerName: dot.ServerName,
			})
		}
	}

	// DoH
	for _, doh := range cfg.UpstreamDoH {
		if doh.Enabled {
			upstreams = append(upstreams, upstream{
				URL:      doh.URL,
				Protocol: "https",
			})
		}
	}

	// UDP Forwarders (Legacy)
	for _, fwd := range cfg.Forwarders {
		upstreams = append(upstreams, upstream{
			Addr:     fwd,
			Protocol: "udp",
		})
	}

	return upstreams
}
