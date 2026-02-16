// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ips

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"grimm.is/flywall/internal/logging"
)

// PatternMatcher performs pattern matching on packet payloads
type PatternMatcher struct {
	// Pattern databases
	signatures  map[uint32]*Signature
	domainRules map[string]*DomainRule
	ipRules     map[uint32]*IPRule

	// Compiled regex patterns
	regexCache map[string]*regexp.Regexp
	regexMutex sync.RWMutex

	// Statistics
	stats      *PatternStats
	statsMutex sync.RWMutex

	// Configuration
	config *PatternConfig

	// State
	enabled bool
	mutex   sync.RWMutex
	logger  *logging.Logger
}

// PatternConfig for pattern matching
type PatternConfig struct {
	Enabled        bool          `json:"enabled"`
	MaxSignatures  int           `json:"max_signatures"`
	MaxDomainRules int           `json:"max_domain_rules"`
	MaxIPRules     int           `json:"max_ip_rules"`
	RegexCacheSize int           `json:"regex_cache_size"`
	MatchTimeout   time.Duration `json:"match_timeout"`
	UpdateInterval time.Duration `json:"update_interval"`
}

// DefaultPatternConfig returns default pattern configuration
func DefaultPatternConfig() *PatternConfig {
	return &PatternConfig{
		Enabled:        true,
		MaxSignatures:  10000,
		MaxDomainRules: 5000,
		MaxIPRules:     10000,
		RegexCacheSize: 1000,
		MatchTimeout:   10 * time.Millisecond,
		UpdateInterval: 1 * time.Hour,
	}
}

// Signature represents a pattern signature
type Signature struct {
	ID          uint32 `json:"id"`
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Type        string `json:"type"` // "regex", "literal", "binary"
	Severity    int    `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`

	// Compiled pattern
	compiled interface{}
}

// DomainRule represents a domain-based rule
type DomainRule struct {
	ID       uint32        `json:"id"`
	Domain   string        `json:"domain"`
	Action   string        `json:"action"` // "allow", "block", "monitor"
	Severity int           `json:"severity"`
	TTL      time.Duration `json:"ttl"`
	Enabled  bool          `json:"enabled"`
}

// IPRule represents an IP-based rule
type IPRule struct {
	ID       uint32        `json:"id"`
	IP       uint32        `json:"ip"`
	Netmask  uint32        `json:"netmask"`
	Action   string        `json:"action"`
	Severity int           `json:"severity"`
	TTL      time.Duration `json:"ttl"`
	Enabled  bool          `json:"enabled"`
}

// PatternStats tracks pattern matching statistics
type PatternStats struct {
	PacketsMatched    uint64        `json:"packets_matched"`
	SignaturesMatched uint64        `json:"signatures_matched"`
	DomainMatches     uint64        `json:"domain_matches"`
	IPMatches         uint64        `json:"ip_matches"`
	FalsePositives    uint64        `json:"false_positives"`
	AvgMatchTime      time.Duration `json:"avg_match_time"`
	LastUpdate        time.Time     `json:"last_update"`
}

// MatchResult represents a pattern match result
type MatchResult struct {
	Matched     bool          `json:"matched"`
	RuleIDs     []uint32      `json:"rule_ids"`
	Severity    int           `json:"severity"`
	Category    string        `json:"category"`
	Description string        `json:"description"`
	Action      string        `json:"action"`
	MatchTime   time.Duration `json:"match_time"`
}

// NewPatternMatcher creates a new pattern matcher
func NewPatternMatcher(logger *logging.Logger, config *PatternConfig) *PatternMatcher {
	if config == nil {
		config = DefaultPatternConfig()
	}

	pm := &PatternMatcher{
		signatures:  make(map[uint32]*Signature),
		domainRules: make(map[string]*DomainRule),
		ipRules:     make(map[uint32]*IPRule),
		regexCache:  make(map[string]*regexp.Regexp),
		stats:       &PatternStats{},
		config:      config,
		enabled:     config.Enabled,
		logger:      logger,
	}

	return pm
}

// AddSignature adds a signature to the matcher
func (pm *PatternMatcher) AddSignature(sig *Signature) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if len(pm.signatures) >= pm.config.MaxSignatures {
		return fmt.Errorf("maximum signatures limit reached")
	}

	// Compile pattern based on type
	switch sig.Type {
	case "regex":
		regex, err := pm.compileRegex(sig.Pattern)
		if err != nil {
			return fmt.Errorf("failed to compile regex: %w", err)
		}
		sig.compiled = regex
	case "literal":
		sig.compiled = sig.Pattern
	case "binary":
		// Convert hex string to bytes
		sig.compiled = []byte(sig.Pattern)
	default:
		return fmt.Errorf("unknown signature type: %s", sig.Type)
	}

	pm.signatures[sig.ID] = sig
	pm.logger.Debug("Added signature", "id", sig.ID, "name", sig.Name, "type", sig.Type)

	return nil
}

// AddDomainRule adds a domain-based rule
func (pm *PatternMatcher) AddDomainRule(rule *DomainRule) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if len(pm.domainRules) >= pm.config.MaxDomainRules {
		return fmt.Errorf("maximum domain rules limit reached")
	}

	pm.domainRules[rule.Domain] = rule
	pm.logger.Debug("Added domain rule", "domain", rule.Domain, "action", rule.Action)

	return nil
}

// AddIPRule adds an IP-based rule
func (pm *PatternMatcher) AddIPRule(rule *IPRule) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if len(pm.ipRules) >= pm.config.MaxIPRules {
		return fmt.Errorf("maximum IP rules limit reached")
	}

	pm.ipRules[rule.IP] = rule
	pm.logger.Debug("Added IP rule", "ip", int2ip(rule.IP), "action", rule.Action)

	return nil
}

// MatchPacket matches a packet against all patterns
func (pm *PatternMatcher) MatchPacket(packet *PacketData) *MatchResult {
	pm.mutex.RLock()
	enabled := pm.enabled
	pm.mutex.RUnlock()

	if !enabled {
		return &MatchResult{Matched: false}
	}

	start := time.Now()
	result := &MatchResult{
		RuleIDs: make([]uint32, 0),
	}

	// Check IP rules
	if ipRule := pm.matchIPRules(packet.SrcIP, packet.DstIP); ipRule != nil {
		result.RuleIDs = append(result.RuleIDs, ipRule.ID)
		result.Severity = max(result.Severity, ipRule.Severity)
		result.Action = ipRule.Action
		result.Matched = true
	}

	// Check domain rules (if DNS info available)
	if packet.Domain != "" {
		if domainRule := pm.matchDomainRules(packet.Domain); domainRule != nil {
			result.RuleIDs = append(result.RuleIDs, domainRule.ID)
			result.Severity = max(result.Severity, domainRule.Severity)
			result.Action = domainRule.Action
			result.Matched = true
		}
	}

	// Check payload signatures
	if packet.Payload != nil && len(packet.Payload) > 0 {
		if sigMatches := pm.matchSignatures(packet.Payload); len(sigMatches) > 0 {
			for _, sig := range sigMatches {
				result.RuleIDs = append(result.RuleIDs, sig.ID)
				result.Severity = max(result.Severity, sig.Severity)
				if result.Category == "" {
					result.Category = sig.Category
				}
				if result.Description == "" {
					result.Description = sig.Description
				}
				result.Matched = true
			}
		}
	}

	result.MatchTime = time.Since(start)
	pm.updateStats(result)

	return result
}

// matchIPRules checks IP-based rules
func (pm *PatternMatcher) matchIPRules(srcIP, dstIP uint32) *IPRule {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// Check source IP
	if rule, exists := pm.ipRules[srcIP]; exists && rule.Enabled {
		return rule
	}

	// Check destination IP
	if rule, exists := pm.ipRules[dstIP]; exists && rule.Enabled {
		return rule
	}

	return nil
}

// matchDomainRules checks domain-based rules
func (pm *PatternMatcher) matchDomainRules(domain string) *DomainRule {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// Exact match
	if rule, exists := pm.domainRules[domain]; exists && rule.Enabled {
		return rule
	}

	// Wildcard match
	for d, rule := range pm.domainRules {
		if !rule.Enabled {
			continue
		}
		if pm.matchWildcard(d, domain) {
			return rule
		}
	}

	return nil
}

// matchSignatures checks payload signatures
func (pm *PatternMatcher) matchSignatures(payload []byte) []*Signature {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var matches []*Signature

	for _, sig := range pm.signatures {
		if !sig.Enabled {
			continue
		}

		if pm.matchSignature(sig, payload) {
			matches = append(matches, sig)
		}
	}

	return matches
}

// matchSignature matches a single signature
func (pm *PatternMatcher) matchSignature(sig *Signature, payload []byte) bool {
	switch sig.Type {
	case "regex":
		if regex, ok := sig.compiled.(*regexp.Regexp); ok {
			return regex.Match(payload)
		}
	case "literal":
		if pattern, ok := sig.compiled.(string); ok {
			return pm.containsPattern(payload, []byte(pattern))
		}
	case "binary":
		if pattern, ok := sig.compiled.([]byte); ok {
			return pm.containsPattern(payload, pattern)
		}
	}

	return false
}

// compileRegex compiles a regex pattern with caching
func (pm *PatternMatcher) compileRegex(pattern string) (*regexp.Regexp, error) {
	pm.regexMutex.RLock()
	if regex, exists := pm.regexCache[pattern]; exists {
		pm.regexMutex.RUnlock()
		return regex, nil
	}
	pm.regexMutex.RUnlock()

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	pm.regexMutex.Lock()
	if len(pm.regexCache) >= pm.config.RegexCacheSize {
		// Remove oldest entry (simple LRU)
		for k := range pm.regexCache {
			delete(pm.regexCache, k)
			break
		}
	}
	pm.regexCache[pattern] = regex
	pm.regexMutex.Unlock()

	return regex, nil
}

// matchWildcard checks if a domain pattern matches (supports wildcards)
func (pm *PatternMatcher) matchWildcard(pattern, domain string) bool {
	// Simple wildcard implementation
	if pattern == "*" {
		return true
	}

	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		if len(domain) >= len(suffix) && domain[len(domain)-len(suffix):] == suffix {
			return true
		}
	}

	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		if len(domain) >= len(prefix) && domain[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// containsPattern checks if payload contains a pattern
func (pm *PatternMatcher) containsPattern(payload, pattern []byte) bool {
	if len(pattern) > len(payload) {
		return false
	}

	for i := 0; i <= len(payload)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if payload[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}

// updateStats updates pattern matching statistics
func (pm *PatternMatcher) updateStats(result *MatchResult) {
	pm.statsMutex.Lock()
	defer pm.statsMutex.Unlock()

	pm.stats.PacketsMatched++

	if result.Matched {
		if result.Category == "signature" {
			pm.stats.SignaturesMatched++
		} else if result.Category == "domain" {
			pm.stats.DomainMatches++
		} else if result.Category == "ip" {
			pm.stats.IPMatches++
		}
	}

	// Update average match time (exponential moving average)
	if pm.stats.AvgMatchTime == 0 {
		pm.stats.AvgMatchTime = result.MatchTime
	} else {
		alpha := 0.1
		pm.stats.AvgMatchTime = time.Duration(
			float64(pm.stats.AvgMatchTime)*(1-alpha) + float64(result.MatchTime)*alpha,
		)
	}

	pm.stats.LastUpdate = time.Now()
}

// GetStatistics returns pattern matching statistics
func (pm *PatternMatcher) GetStatistics() *PatternStats {
	pm.statsMutex.RLock()
	defer pm.statsMutex.RUnlock()

	stats := *pm.stats
	return &stats
}

// SetEnabled enables or disables pattern matching
func (pm *PatternMatcher) SetEnabled(enabled bool) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.enabled = enabled
	pm.logger.Info("Pattern matching", "enabled", enabled)
}

// Clear clears all patterns and rules
func (pm *PatternMatcher) Clear() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.signatures = make(map[uint32]*Signature)
	pm.domainRules = make(map[string]*DomainRule)
	pm.ipRules = make(map[uint32]*IPRule)

	pm.regexMutex.Lock()
	pm.regexCache = make(map[string]*regexp.Regexp)
	pm.regexMutex.Unlock()

	pm.logger.Info("Cleared all patterns and rules")
}

// PacketData represents packet data for pattern matching
type PacketData struct {
	SrcIP    uint32
	DstIP    uint32
	SrcPort  uint16
	DstPort  uint16
	Protocol uint8
	Domain   string // From DNS lookup
	Payload  []byte // Packet payload
	Length   int    // Payload length
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
