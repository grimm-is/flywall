// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ips

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"grimm.is/flywall/internal/logging"
)

// PatternDB manages pattern databases and updates
type PatternDB struct {
	patternMatcher *PatternMatcher
	logger         *logging.Logger
	config         *PatternDBConfig

	// Database state
	signatures     map[uint32]*Signature
	domainRules    map[string]*DomainRule
	ipRules        map[uint32]*IPRule

	// Update state
	lastUpdate     time.Time
	updateMutex    sync.RWMutex
	updateTicker   *time.Ticker
	stopCh         chan struct{}

	// HTTP client for remote updates
	client         *http.Client
}

// PatternDBConfig for pattern database management
type PatternDBConfig struct {
	Enabled          bool          `json:"enabled"`
	LocalDBPath      string        `json:"local_db_path"`
	RemoteURL        string        `json:"remote_url"`
	UpdateInterval   time.Duration `json:"update_interval"`
	UpdateTimeout    time.Duration `json:"update_timeout"`
	AutoUpdate       bool          `json:"auto_update"`
	SignatureSources []string      `json:"signature_sources"`
}

// DefaultPatternDBConfig returns default pattern database configuration
func DefaultPatternDBConfig() *PatternDBConfig {
	return &PatternDBConfig{
		Enabled:        true,
		LocalDBPath:    "/etc/flywall/patterns.db",
		RemoteURL:      "https://updates.flywall.io/patterns",
		UpdateInterval: 24 * time.Hour,
		UpdateTimeout:  30 * time.Second,
		AutoUpdate:     true,
		SignatureSources: []string{
			"https://rules.emergingthreats.net/open/suricata.rules",
			"https://github.com/Neo23x0/signature-base/raw/master/",
		},
	}
}

// PatternDatabase represents a pattern database file
type PatternDatabase struct {
	Version     string      `json:"version"`
	GeneratedAt time.Time   `json:"generated_at"`
	ExpiresAt   time.Time   `json:"expires_at"`
	Signatures  []*Signature `json:"signatures"`
	DomainRules []*DomainRule `json:"domain_rules"`
	IPRules     []*IPRule     `json:"ip_rules"`
}

// NewPatternDB creates a new pattern database manager
func NewPatternDB(patternMatcher *PatternMatcher, logger *logging.Logger, config *PatternDBConfig) *PatternDB {
	if config == nil {
		config = DefaultPatternDBConfig()
	}

	pdb := &PatternDB{
		patternMatcher: patternMatcher,
		logger:         logger,
		config:         config,
		signatures:     make(map[uint32]*Signature),
		domainRules:    make(map[string]*DomainRule),
		ipRules:        make(map[uint32]*IPRule),
		stopCh:         make(chan struct{}),
		client: &http.Client{
			Timeout: config.UpdateTimeout,
		},
	}

	return pdb
}

// Start starts the pattern database manager
func (pdb *PatternDB) Start() error {
	if !pdb.config.Enabled {
		pdb.logger.Info("Pattern database disabled")
		return nil
	}

	// Load local database
	if err := pdb.loadLocalDB(); err != nil {
		pdb.logger.Warn("Failed to load local pattern database", "error", err)
	}

	// Start auto-update if enabled
	if pdb.config.AutoUpdate && pdb.config.UpdateInterval > 0 {
		pdb.updateTicker = time.NewTicker(pdb.config.UpdateInterval)
		go pdb.updateWorker()
		pdb.logger.Info("Pattern database auto-update started",
			"interval", pdb.config.UpdateInterval)
	}

	return nil
}

// Stop stops the pattern database manager
func (pdb *PatternDB) Stop() {
	if pdb.updateTicker != nil {
		pdb.updateTicker.Stop()
	}
	close(pdb.stopCh)
	pdb.logger.Info("Pattern database stopped")
}

// UpdateNow triggers an immediate database update
func (pdb *PatternDB) UpdateNow() error {
	pdb.logger.Info("Starting pattern database update")

	// Download from remote
	db, err := pdb.downloadRemoteDB()
	if err != nil {
		return fmt.Errorf("failed to download remote database: %w", err)
	}

	// Validate database
	if err := pdb.validateDB(db); err != nil {
		return fmt.Errorf("invalid database: %w", err)
	}

	// Apply database
	if err := pdb.applyDB(db); err != nil {
		return fmt.Errorf("failed to apply database: %w", err)
	}

	// Save to local
	if err := pdb.saveLocalDB(db); err != nil {
		pdb.logger.Warn("Failed to save local database", "error", err)
	}

	pdb.updateMutex.Lock()
	pdb.lastUpdate = time.Now()
	pdb.updateMutex.Unlock()

	pdb.logger.Info("Pattern database updated successfully",
		"signatures", len(db.Signatures),
		"domain_rules", len(db.DomainRules),
		"ip_rules", len(db.IPRules))

	return nil
}

// loadLocalDB loads the local pattern database
func (pdb *PatternDB) loadLocalDB() error {
	if pdb.config.LocalDBPath == "" {
		return fmt.Errorf("no local database path configured")
	}

	// Check if file exists
	if _, err := os.Stat(pdb.config.LocalDBPath); os.IsNotExist(err) {
		pdb.logger.Info("Local pattern database not found, will create on first update")
		return nil
	}

	// Read file
	data, err := ioutil.ReadFile(pdb.config.LocalDBPath)
	if err != nil {
		return fmt.Errorf("failed to read database file: %w", err)
	}

	// Parse JSON
	var db PatternDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("failed to parse database: %w", err)
	}

	// Validate and apply
	if err := pdb.validateDB(&db); err != nil {
		return fmt.Errorf("invalid local database: %w", err)
	}

	if err := pdb.applyDB(&db); err != nil {
		return fmt.Errorf("failed to apply local database: %w", err)
	}

	pdb.updateMutex.Lock()
	pdb.lastUpdate = time.Now()
	pdb.updateMutex.Unlock()

	pdb.logger.Info("Loaded local pattern database",
		"version", db.Version,
		"signatures", len(db.Signatures))

	return nil
}

// downloadRemoteDB downloads the pattern database from remote URL
func (pdb *PatternDB) downloadRemoteDB() (*PatternDatabase, error) {
	if pdb.config.RemoteURL == "" {
		return nil, fmt.Errorf("no remote URL configured")
	}

	resp, err := pdb.client.Get(pdb.config.RemoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var db PatternDatabase
	if err := json.NewDecoder(resp.Body).Decode(&db); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &db, nil
}

// validateDB validates a pattern database
func (pdb *PatternDB) validateDB(db *PatternDatabase) error {
	if db.Version == "" {
		return fmt.Errorf("missing version")
	}

	if db.GeneratedAt.IsZero() {
		return fmt.Errorf("missing generation time")
	}

	if !db.ExpiresAt.IsZero() && time.Now().After(db.ExpiresAt) {
		return fmt.Errorf("database expired")
	}

	// Validate signatures
	for _, sig := range db.Signatures {
		if sig.ID == 0 {
			return fmt.Errorf("signature missing ID")
		}
		if sig.Pattern == "" {
			return fmt.Errorf("signature %d missing pattern", sig.ID)
		}
		if sig.Type == "" {
			return fmt.Errorf("signature %d missing type", sig.ID)
		}
	}

	return nil
}

// applyDB applies a pattern database to the pattern matcher
func (pdb *PatternDB) applyDB(db *PatternDatabase) error {
	// Clear existing patterns
	pdb.patternMatcher.Clear()

	// Apply signatures
	for _, sig := range db.Signatures {
		if err := pdb.patternMatcher.AddSignature(sig); err != nil {
			pdb.logger.Warn("Failed to add signature", "id", sig.ID, "error", err)
		}
	}

	// Apply domain rules
	for _, rule := range db.DomainRules {
		if err := pdb.patternMatcher.AddDomainRule(rule); err != nil {
			pdb.logger.Warn("Failed to add domain rule", "domain", rule.Domain, "error", err)
		}
	}

	// Apply IP rules
	for _, rule := range db.IPRules {
		if err := pdb.patternMatcher.AddIPRule(rule); err != nil {
			pdb.logger.Warn("Failed to add IP rule", "ip", int2ip(rule.IP), "error", err)
		}
	}

	// Update internal state
	pdb.signatures = make(map[uint32]*Signature)
	for _, sig := range db.Signatures {
		pdb.signatures[sig.ID] = sig
	}

	pdb.domainRules = make(map[string]*DomainRule)
	for _, rule := range db.DomainRules {
		pdb.domainRules[rule.Domain] = rule
	}

	pdb.ipRules = make(map[uint32]*IPRule)
	for _, rule := range db.IPRules {
		pdb.ipRules[rule.IP] = rule
	}

	return nil
}

// saveLocalDB saves the pattern database to local file
func (pdb *PatternDB) saveLocalDB(db *PatternDatabase) error {
	if pdb.config.LocalDBPath == "" {
		return nil
	}

	// Create directory if needed
	dir := filepath.Dir(pdb.config.LocalDBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal JSON
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal database: %w", err)
	}

	// Write to temporary file
	tmpFile := pdb.config.LocalDBPath + ".tmp"
	if err := ioutil.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, pdb.config.LocalDBPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// updateWorker handles periodic updates
func (pdb *PatternDB) updateWorker() {
	for {
		select {
		case <-pdb.stopCh:
			return
		case <-pdb.updateTicker.C:
			if err := pdb.UpdateNow(); err != nil {
				pdb.logger.Error("Failed to update pattern database", "error", err)
			}
		}
	}
}

// GetStatus returns the current status of the pattern database
func (pdb *PatternDB) GetStatus() *PatternDBStatus {
	pdb.updateMutex.RLock()
	lastUpdate := pdb.lastUpdate
	pdb.updateMutex.RUnlock()

	stats := pdb.patternMatcher.GetStatistics()

	return &PatternDBStatus{
		Enabled:         pdb.config.Enabled,
		LastUpdate:      lastUpdate,
		NextUpdate:      lastUpdate.Add(pdb.config.UpdateInterval),
		SignaturesCount: len(pdb.signatures),
		DomainRulesCount: len(pdb.domainRules),
		IPRulesCount:    len(pdb.ipRules),
		Statistics:      stats,
	}
}

// PatternDBStatus represents the status of the pattern database
type PatternDBStatus struct {
	Enabled          bool           `json:"enabled"`
	LastUpdate       time.Time      `json:"last_update"`
	NextUpdate       time.Time      `json:"next_update"`
	SignaturesCount  int            `json:"signatures_count"`
	DomainRulesCount int            `json:"domain_rules_count"`
	IPRulesCount     int            `json:"ip_rules_count"`
	Statistics       *PatternStats  `json:"statistics"`
}

// ImportRules imports rules from various formats
func (pdb *PatternDB) ImportRules(source string, format string, data []byte) error {
	pdb.logger.Info("Importing rules", "source", source, "format", format)

	switch format {
	case "suricata":
		return pdb.importSuricataRules(data)
	case "snort":
		return pdb.importSnortRules(data)
	case "json":
		return pdb.importJSONRules(data)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// importSuricataRules imports Suricata rules
func (pdb *PatternDB) importSuricataRules(data []byte) error {
	// TODO: Implement Suricata rule parser
	// This would parse the rule format and convert to signatures
	return fmt.Errorf("Suricata import not yet implemented")
}

// importSnortRules imports Snort rules
func (pdb *PatternDB) importSnortRules(data []byte) error {
	// TODO: Implement Snort rule parser
	// This would parse the rule format and convert to signatures
	return fmt.Errorf("Snort import not yet implemented")
}

// importJSONRules imports rules in JSON format
func (pdb *PatternDB) importJSONRules(data []byte) error {
	var db PatternDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return pdb.applyDB(&db)
}
