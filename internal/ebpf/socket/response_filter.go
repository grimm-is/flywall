// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// ResponseFilter handles DNS response filtering
type ResponseFilter struct {
	// Configuration
	config *ResponseFilterConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Filter lists
	blocklist     *DomainList
	allowlist     *DomainList
	maliciousList *DomainList

	// Response cache for tracking
	responseCache map[string]*ResponseCacheEntry
	cacheMutex    sync.RWMutex

	// Statistics
	stats *ResponseFilterStats

	// Event handlers
	blockHandler DNSResponseBlockHandler
	allowHandler DNSResponseAllowHandler

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// ResponseFilterConfig holds configuration for response filtering
type ResponseFilterConfig struct {
	// Filter settings
	Enabled         bool `hcl:"enabled,optional"`
	BlockMalicious  bool `hcl:"block_malicious,optional"`
	BlockPrivateDNS bool `hcl:"block_private_dns,optional"`
	AllowlistOnly   bool `hcl:"allowlist_only,optional"`

	// List settings
	BlocklistSources []string      `hcl:"blocklist_sources,optional"`
	AllowlistSources []string      `hcl:"allowlist_sources,optional"`
	MaliciousSources []string      `hcl:"malicious_sources,optional"`
	UpdateInterval   time.Duration `hcl:"update_interval,optional"`

	// Cache settings
	CacheEnabled bool          `hcl:"cache_enabled,optional"`
	CacheSize    int           `hcl:"cache_size,optional"`
	CacheTTL     time.Duration `hcl:"cache_ttl,optional"`

	// Response validation
	ValidateResponses bool   `hcl:"validate_responses,optional"`
	MaxResponseSize   int    `hcl:"max_response_size,optional"`
	MinTTL            uint32 `hcl:"min_ttl,optional"`

	// Filtering rules
	BlockNXDOMAIN       bool     `hcl:"block_nxdomain,optional"`
	BlockSERVFAIL       bool     `hcl:"block_servfail,optional"`
	BlockEmptyResponses bool     `hcl:"block_empty_responses,optional"`
	MaxAnswers          int      `hcl:"max_answers,optional"`
	RegexPatterns       []string `hcl:"regex_patterns,optional"`
}

// ResponseFilterStats holds statistics for response filtering
type ResponseFilterStats struct {
	ResponsesChecked uint64    `json:"responses_checked"`
	ResponsesBlocked uint64    `json:"responses_blocked"`
	ResponsesAllowed uint64    `json:"responses_allowed"`
	BlocklistHits    uint64    `json:"blocklist_hits"`
	AllowlistHits    uint64    `json:"allowlist_hits"`
	MaliciousHits    uint64    `json:"malicious_hits"`
	CacheHits        uint64    `json:"cache_hits"`
	CacheMisses      uint64    `json:"cache_misses"`
	ValidationErrors uint64    `json:"validation_errors"`
	LastUpdate       time.Time `json:"last_update"`
}

// ResponseCacheEntry holds cached response information
type ResponseCacheEntry struct {
	Domain       string
	ResponseCode uint8
	Blocked      bool
	Reason       string
	Timestamp    time.Time
	TTL          uint32
}

// DomainList represents a list of domains
type DomainList struct {
	mutex   sync.RWMutex
	domains map[string]bool
	rules   []*DomainRule
}

// DomainRule represents a domain filtering rule
type DomainRule struct {
	Pattern       string     `json:"pattern"`
	IsRegex       bool       `json:"is_regex"`
	Action        string     `json:"action"` // block, allow
	Reason        string     `json:"reason"`
	Expiry        *time.Time `json:"expiry,omitempty"`
	compiledRegex *regexp.Regexp
}

// DNSResponseBlockHandler handles blocked responses
type DNSResponseBlockHandler func(event *types.DNSResponseEvent, reason string) error

// DNSResponseAllowHandler handles allowed responses
type DNSResponseAllowHandler func(event *types.DNSResponseEvent) error

// DefaultResponseFilterConfig returns default configuration
func DefaultResponseFilterConfig() *ResponseFilterConfig {
	return &ResponseFilterConfig{
		Enabled:             false,
		BlockMalicious:      true,
		BlockPrivateDNS:     false,
		AllowlistOnly:       false,
		UpdateInterval:      1 * time.Hour,
		CacheEnabled:        true,
		CacheSize:           10000,
		CacheTTL:            5 * time.Minute,
		ValidateResponses:   true,
		MaxResponseSize:     4096,
		MinTTL:              60,
		BlockNXDOMAIN:       false,
		BlockSERVFAIL:       false,
		BlockEmptyResponses: false,
		MaxAnswers:          100,
	}
}

// NewResponseFilter creates a new response filter
func NewResponseFilter(logger *logging.Logger, config *ResponseFilterConfig) *ResponseFilter {
	if config == nil {
		config = DefaultResponseFilterConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	rf := &ResponseFilter{
		config:        config,
		stats:         &ResponseFilterStats{LastUpdate: time.Now()},
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		responseCache: make(map[string]*ResponseCacheEntry),
		blocklist:     NewDomainList(),
		allowlist:     NewDomainList(),
		maliciousList: NewDomainList(),
	}

	return rf
}

// Start starts the response filter
func (rf *ResponseFilter) Start() error {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	if !rf.config.Enabled {
		rf.logger.Info("DNS response filter disabled")
		return nil
	}

	rf.logger.Info("Starting DNS response filter")

	// Load domain lists
	if err := rf.loadDomainLists(); err != nil {
		return err
	}

	// Start update goroutine
	go rf.updateWorker()

	// Start cache cleanup
	if rf.config.CacheEnabled {
		go rf.cacheCleanupWorker()
	}

	rf.enabled = true
	rf.logger.Info("DNS response filter started",
		"blocklist_size", rf.blocklist.Size(),
		"allowlist_size", rf.allowlist.Size(),
		"malicious_size", rf.maliciousList.Size())

	return nil
}

// Stop stops the response filter
func (rf *ResponseFilter) Stop() {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	if !rf.enabled {
		return
	}

	rf.logger.Info("Stopping DNS response filter")

	// Cancel context
	rf.cancel()

	rf.enabled = false
	rf.logger.Info("DNS response filter stopped")
}

// FilterResponse filters a DNS response
func (rf *ResponseFilter) FilterResponse(event *types.DNSResponseEvent) (bool, string) {
	if !rf.enabled {
		return true, "filter disabled"
	}

	atomic.AddUint64(&rf.stats.ResponsesChecked, 1)
	rf.stats.LastUpdate = time.Now()

	// Check cache first
	if rf.config.CacheEnabled {
		if blocked, reason := rf.checkCache(event); reason != "" {
			if blocked {
				atomic.AddUint64(&rf.stats.ResponsesBlocked, 1)
				atomic.AddUint64(&rf.stats.CacheHits, 1)
				return false, reason
			}
			atomic.AddUint64(&rf.stats.ResponsesAllowed, 1)
			atomic.AddUint64(&rf.stats.CacheHits, 1)
			return true, "cache allow"
		}
		atomic.AddUint64(&rf.stats.CacheMisses, 1)
	}

	// Validate response
	if rf.config.ValidateResponses {
		if !rf.validateResponse(event) {
			atomic.AddUint64(&rf.stats.ResponsesBlocked, 1)
			atomic.AddUint64(&rf.stats.ValidationErrors, 1)
			reason := "validation failed"
			rf.updateCache(event, true, reason)
			return false, reason
		}
	}

	// Check filtering rules
	blocked, reason := rf.checkFilteringRules(event)

	// Update cache
	if rf.config.CacheEnabled {
		rf.updateCache(event, blocked, reason)
	}

	// Update statistics
	if blocked {
		atomic.AddUint64(&rf.stats.ResponsesBlocked, 1)

		// Call block handler
		if rf.blockHandler != nil {
			rf.blockHandler(event, reason)
		}
	} else {
		atomic.AddUint64(&rf.stats.ResponsesAllowed, 1)

		// Call allow handler
		if rf.allowHandler != nil {
			rf.allowHandler(event)
		}
	}

	return !blocked, reason
}

// checkCache checks the response cache
func (rf *ResponseFilter) checkCache(event *types.DNSResponseEvent) (bool, string) {
	rf.cacheMutex.RLock()
	defer rf.cacheMutex.RUnlock()

	entry, exists := rf.responseCache[event.Domain]
	if !exists {
		return false, ""
	}

	// Check if cache entry is still valid
	if time.Since(entry.Timestamp) > rf.config.CacheTTL {
		return false, ""
	}

	return entry.Blocked, entry.Reason
}

// updateCache updates the response cache
func (rf *ResponseFilter) updateCache(event *types.DNSResponseEvent, blocked bool, reason string) {
	if !rf.config.CacheEnabled {
		return
	}

	rf.cacheMutex.Lock()
	defer rf.cacheMutex.Unlock()

	// Clean up cache if it's too large
	if len(rf.responseCache) >= rf.config.CacheSize {
		rf.cleanupCache()
	}

	// Find minimum TTL from answers
	ttl := rf.config.MinTTL
	if len(event.Answers) > 0 {
		minTTL := uint32(0)
		for i, ans := range event.Answers {
			if i == 0 || ans.TTL < minTTL {
				minTTL = ans.TTL
			}
		}
		if minTTL > 0 {
			ttl = minTTL
		}
	}

	rf.responseCache[event.Domain] = &ResponseCacheEntry{
		Domain:       event.Domain,
		ResponseCode: event.ResponseCode,
		Blocked:      blocked,
		Reason:       reason,
		Timestamp:    time.Now(),
		TTL:          ttl,
	}
}

// validateResponse validates a DNS response
func (rf *ResponseFilter) validateResponse(event *types.DNSResponseEvent) bool {
	// Check response size
	if rf.config.MaxResponseSize > 0 && int(event.PacketSize) > rf.config.MaxResponseSize {
		rf.logger.Debug("Response too large", "domain", event.Domain, "size", event.PacketSize)
		return false
	}

	// Check response codes
	if rf.config.BlockNXDOMAIN && event.ResponseCode == 3 { // NXDOMAIN
		rf.logger.Debug("Blocking NXDOMAIN response", "domain", event.Domain)
		return false
	}

	if rf.config.BlockSERVFAIL && event.ResponseCode == 2 { // SERVFAIL
		rf.logger.Debug("Blocking SERVFAIL response", "domain", event.Domain)
		return false
	}

	// Check empty responses
	if rf.config.BlockEmptyResponses && event.AnswerCount == 0 && event.ResponseCode == 0 {
		rf.logger.Debug("Blocking empty response", "domain", event.Domain)
		return false
	}

	// Check answer count
	if rf.config.MaxAnswers > 0 && int(event.AnswerCount) > rf.config.MaxAnswers {
		rf.logger.Debug("Too many answers", "domain", event.Domain, "count", event.AnswerCount)
		return false
	}

	return true
}

// checkFilteringRules checks filtering rules
func (rf *ResponseFilter) checkFilteringRules(event *types.DNSResponseEvent) (bool, string) {
	domain := event.Domain

	// Check allowlist first (if in allowlist-only mode)
	if rf.config.AllowlistOnly {
		if rf.allowlist.Contains(domain) {
			atomic.AddUint64(&rf.stats.AllowlistHits, 1)
			return false, "allowlisted"
		}
		return true, "not in allowlist"
	}

	// Check malicious domains
	if rf.config.BlockMalicious && rf.maliciousList.Contains(domain) {
		atomic.AddUint64(&rf.stats.MaliciousHits, 1)
		return true, "malicious domain"
	}

	// Check blocklist
	if rf.blocklist.Contains(domain) {
		atomic.AddUint64(&rf.stats.BlocklistHits, 1)
		return true, "blocked domain"
	}

	// Check private DNS
	if rf.config.BlockPrivateDNS && rf.isPrivateDNSResponse(event) {
		return true, "private DNS response"
	}

	return false, "allowed"
}

// isPrivateDNSResponse checks if this is a private DNS response
func (rf *ResponseFilter) isPrivateDNSResponse(event *types.DNSResponseEvent) bool {
	for _, ans := range event.Answers {
		if ans.Type == types.DNSTypeA || ans.Type == types.DNSTypeAAAA {
			ip := net.ParseIP(ans.Data)
			if ip != nil && ip.IsPrivate() {
				return true
			}
		}
	}
	return false
}

// loadDomainLists loads domain lists from sources
func (rf *ResponseFilter) loadDomainLists() error {
	// Load blocklist
	for _, source := range rf.config.BlocklistSources {
		if err := rf.fetchList(source, rf.blocklist, "block", "blocked by source: "+source); err != nil {
			rf.logger.Warn("Failed to load blocklist source", "source", source, "error", err)
		}
	}

	// Load regex patterns from config
	for _, pattern := range rf.config.RegexPatterns {
		if err := rf.blocklist.AddRule(pattern, true, "block", "configured regex block"); err != nil {
			rf.logger.Warn("Failed to compile regex pattern from config", "pattern", pattern, "error", err)
		}
	}

	// Load allowlist
	for _, source := range rf.config.AllowlistSources {
		if err := rf.fetchList(source, rf.allowlist, "allow", "allowed by source: "+source); err != nil {
			rf.logger.Warn("Failed to load allowlist source", "source", source, "error", err)
		}
	}

	// Load malicious list
	for _, source := range rf.config.MaliciousSources {
		if err := rf.fetchList(source, rf.maliciousList, "block", "malicious domain from: "+source); err != nil {
			rf.logger.Warn("Failed to load malicious source", "source", source, "error", err)
		}
	}

	return nil
}

// fetchList fetches a domain list from a URL or local file
func (rf *ResponseFilter) fetchList(source string, list *DomainList, action string, reason string) error {
	var data []byte
	var err error

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		rf.logger.Debug("Fetching domain list from URL", "url", source)

		// Create client with timeout
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		resp, err := client.Get(source)
		if err != nil {
			return fmt.Errorf("failed to fetch list from URL: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
	} else {
		// Load from local file
		rf.logger.Debug("Loading domain list from file", "path", source)
		data, err = os.ReadFile(source)
		if err != nil {
			return err
		}
	}

	// Parse domain list (one domain per line)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if it's a regex rule (enclosed in / /)
		if strings.HasPrefix(line, "/") && strings.HasSuffix(line, "/") && len(line) > 2 {
			pattern := line[1 : len(line)-1]
			list.AddRule(pattern, true, action, reason)
		} else {
			list.AddDomain(line)
		}
	}

	return nil
}

// updateWorker periodically updates domain lists
func (rf *ResponseFilter) updateWorker() {
	ticker := time.NewTicker(rf.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rf.ctx.Done():
			return
		case <-ticker.C:
			if err := rf.loadDomainLists(); err != nil {
				rf.logger.Error("Failed to update domain lists", "error", err)
			}
		}
	}
}

// cacheCleanupWorker cleans up expired cache entries
func (rf *ResponseFilter) cacheCleanupWorker() {
	ticker := time.NewTicker(rf.config.CacheTTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-rf.ctx.Done():
			return
		case <-ticker.C:
			rf.cleanupCache()
		}
	}
}

// cleanupCache removes expired cache entries
func (rf *ResponseFilter) cleanupCache() {
	rf.cacheMutex.Lock()
	defer rf.cacheMutex.Unlock()

	now := time.Now()
	for domain, entry := range rf.responseCache {
		if now.Sub(entry.Timestamp) > rf.config.CacheTTL {
			delete(rf.responseCache, domain)
		}
	}
}

// SetBlockHandler sets the block handler
func (rf *ResponseFilter) SetBlockHandler(handler func(event *types.DNSResponseEvent, reason string) error) {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()
	rf.blockHandler = handler
}

// SetAllowHandler sets the allow handler
func (rf *ResponseFilter) SetAllowHandler(handler func(event *types.DNSResponseEvent) error) {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()
	rf.allowHandler = handler
}

// GetStatistics returns response filter statistics
func (rf *ResponseFilter) GetStatistics() interface{} {
	rf.mutex.RLock()
	defer rf.mutex.RUnlock()

	stats := *rf.stats
	return &stats
}

// IsEnabled returns whether the filter is enabled
func (rf *ResponseFilter) IsEnabled() bool {
	rf.mutex.RLock()
	defer rf.mutex.RUnlock()
	return rf.enabled
}

// NewDomainList creates a new domain list
func NewDomainList() *DomainList {
	return &DomainList{
		domains: make(map[string]bool),
		rules:   make([]*DomainRule, 0),
	}
}

// AddDomain adds a domain to the list
func (dl *DomainList) AddDomain(domain string) {
	dl.mutex.Lock()
	defer dl.mutex.Unlock()
	dl.domains[domain] = true
}

// AddRule adds a filtering rule to the list
func (dl *DomainList) AddRule(pattern string, isRegex bool, action string, reason string) error {
	dl.mutex.Lock()
	defer dl.mutex.Unlock()

	rule := &DomainRule{
		Pattern: pattern,
		IsRegex: isRegex,
		Action:  action,
		Reason:  reason,
	}

	if isRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
		rule.compiledRegex = re
	}

	dl.rules = append(dl.rules, rule)
	return nil
}

// RemoveDomain removes a domain from the list
func (dl *DomainList) RemoveDomain(domain string) {
	dl.mutex.Lock()
	defer dl.mutex.Unlock()
	delete(dl.domains, domain)
}

// Contains checks if a domain is in the list
func (dl *DomainList) Contains(domain string) bool {
	dl.mutex.RLock()
	defer dl.mutex.RUnlock()

	// Check exact match
	if dl.domains[domain] {
		return true
	}

	// Check rules
	for _, rule := range dl.rules {
		if rule.matches(domain) {
			return true
		}
	}

	return false
}

// Size returns the number of domains in the list
func (dl *DomainList) Size() int {
	dl.mutex.RLock()
	defer dl.mutex.RUnlock()
	return len(dl.domains)
}

// matches checks if a rule matches a domain
func (dr *DomainRule) matches(domain string) bool {
	if dr.IsRegex {
		if dr.compiledRegex != nil {
			return dr.compiledRegex.MatchString(domain)
		}
		// Fallback if not pre-compiled (should not happen with AddRule)
		matched, _ := regexp.MatchString(dr.Pattern, domain)
		return matched
	}
	return dr.Pattern == domain
}
