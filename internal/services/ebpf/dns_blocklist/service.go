// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dns_blocklist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"grimm.is/flywall/internal/ebpf/interfaces"
	"grimm.is/flywall/internal/install"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/services/dns"
)

// Service manages DNS blocklist functionality
type Service struct {
	loader interfaces.Loader
	logger *logging.Logger
	config *Config

	// Runtime state
	bloomFilter *ebpf.Map
	domains     map[string]bool
	mutex       sync.RWMutex
	running     bool
	stopCh      chan struct{}
	lastUpdate  time.Time
}

// Config defines DNS blocklist configuration
type Config struct {
	// Bloom filter settings
	BloomSize uint32 `json:"bloom_size"` // Size of bloom filter
	HashCount uint32 `json:"hash_count"` // Number of hash functions

	// Blocklist sources
	Sources []string `json:"sources"` // URLs or file paths

	// Update settings
	UpdateInterval time.Duration `json:"update_interval"`

	// Performance settings
	MaxDomains      int           `json:"max_domains"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// NewService creates a new DNS blocklist service
func NewService(loader interfaces.Loader, logger *logging.Logger, config *Config) *Service {
	return &Service{
		loader:  loader,
		logger:  logger,
		config:  config,
		domains: make(map[string]bool),
		stopCh:  make(chan struct{}),
	}
}

// Start initializes the DNS blocklist service
func (s *Service) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("DNS blocklist service already running")
	}

	// Initialize bloom filter map
	bloomMap, err := s.loader.GetMap("dns_bloom")
	if err != nil {
		return fmt.Errorf("failed to get dns_bloom map: %w", err)
	}
	s.bloomFilter = bloomMap.GetMap()

	// Load initial blocklist
	if err := s.loadBlocklists(); err != nil {
		s.logger.Error("Failed to load initial blocklists", "error", err)
		// Continue anyway - we can run with empty blocklist
	}

	// Start update routine
	go s.updateRoutine()

	s.running = true
	s.logger.Info("DNS blocklist service started")

	return nil
}

// Stop stops the DNS blocklist service
func (s *Service) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return nil
	}

	close(s.stopCh)
	s.running = false
	s.logger.Info("DNS blocklist service stopped")

	return nil
}

// AddDomain adds a domain to the blocklist
func (s *Service) AddDomain(domain string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.domains[domain] {
		return nil // Already exists
	}

	// Add to bloom filter
	if err := s.addToBloomFilter(domain); err != nil {
		return fmt.Errorf("failed to add domain to bloom filter: %w", err)
	}

	s.domains[domain] = true
	s.logger.Debug("Added domain to blocklist", "domain", domain)

	return nil
}

// RemoveDomain removes a domain from the blocklist
func (s *Service) RemoveDomain(domain string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.domains[domain] {
		return nil // Doesn't exist
	}

	// Note: Can't remove from bloom filter, need to rebuild
	delete(s.domains, domain)
	s.logger.Debug("Removed domain from blocklist", "domain", domain)

	// Schedule bloom filter rebuild
	go s.rebuildBloomFilter()

	return nil
}

// IsBlocked checks if a domain is blocked
func (s *Service) IsBlocked(domain string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.domains[domain]
}

// GetStats returns blocklist statistics
func (s *Service) GetStats() *interfaces.Stats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return &interfaces.Stats{
		DomainCount:    len(s.domains),
		BloomSize:      s.config.BloomSize,
		HashCount:      s.config.HashCount,
		SourceCount:    len(s.config.Sources),
		LastUpdate:     s.lastUpdate,
		UpdateInterval: s.config.UpdateInterval,
	}
}

// Export returns the current blocklist
func (s *Service) Export() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]string, 0, len(s.domains))
	for domain := range s.domains {
		result = append(result, domain)
	}

	return result
}

// Import imports a list of domains
func (s *Service) Import(domains []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	added := 0
	for _, domain := range domains {
		if !s.domains[domain] {
			if err := s.addToBloomFilter(domain); err != nil {
				s.logger.Error("Failed to add domain during import", "domain", domain, "error", err)
				continue
			}
			s.domains[domain] = true
			added++
		}
	}

	// Update last update time
	s.lastUpdate = time.Now()

	s.logger.Info("Imported domains", "total", len(domains), "added", added)
	return nil
}

// UpdateConfig updates the service configuration
func (s *Service) UpdateConfig(newConfig *Config) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Update configuration
	s.config = newConfig

	// Restart update routine with new interval
	if s.running {
		// Stop current routine
		close(s.stopCh)
		s.stopCh = make(chan struct{})

		// Start new routine
		go s.updateRoutine()
	}

	s.logger.Info("Configuration updated", "update_interval", s.config.UpdateInterval)
	return nil
}

// Clear removes all domains from the blocklist
func (s *Service) Clear() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Clear local domain list
	s.domains = make(map[string]bool)

	// Clear bloom filter if available
	if s.bloomFilter != nil {
		var key uint32
		var zero uint8
		for i := uint32(0); i < 131072; i++ {
			key = i
			s.bloomFilter.Update(&key, &zero, ebpf.UpdateAny)
		}
		s.logger.Info("Cleared DNS blocklist")
	}

	return nil
}

// parseBlocklist parses domains from a reader
func parseBlocklist(r io.Reader) ([]string, error) {
	var domains []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle inline comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		if len(parts) >= 2 {
			// Hosts file format: IP domain [domain2...]
			for _, domain := range parts[1:] {
				domain = strings.ToLower(domain)
				if domain != "localhost" && domain != "localhost.localdomain" {
					domains = append(domains, domain)
				}
			}
		} else {
			// Plain domain format
			domain := strings.ToLower(parts[0])
			if domain != "localhost" && domain != "localhost.localdomain" {
				domains = append(domains, domain)
			}
		}
	}

	return domains, nil
}

// loadBlocklists loads domains from all configured sources
func (s *Service) loadBlocklists() error {
	for _, source := range s.config.Sources {
		if err := s.loadFromSource(source); err != nil {
			s.logger.Error("Failed to load from source", "source", source, "error", err)
		}
	}
	return nil
}

// loadFromSource loads domains from a single source
func (s *Service) loadFromSource(source string) error {
	var domains []string
	var err error

	// Check if source is a URL or file path
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		// Use existing DNS service to download with caching
		cachePath := filepath.Join(install.GetStateDir(), "cache", "dns-blocklist")
		domains, err = dns.DownloadBlocklistWithCache(source, cachePath)
		if err != nil {
			return fmt.Errorf("failed to download blocklist from %s: %w", source, err)
		}
	} else {
		// Load from local file
		domains, err = dns.LoadCachedBlocklist(filepath.Dir(source), source)
		if err != nil {
			// Try to open as a regular file
			f, err := os.Open(source)
			if err != nil {
				return fmt.Errorf("failed to open blocklist file %s: %w", source, err)
			}
			defer f.Close()
			domains, err = parseBlocklist(f)
			if err != nil {
				return fmt.Errorf("failed to parse blocklist file %s: %w", source, err)
			}
		}
	}

	// Add domains to bloom filter
	added := 0
	for _, domain := range domains {
		if !s.domains[domain] {
			if err := s.addToBloomFilter(domain); err != nil {
				s.logger.Error("Failed to add domain to bloom filter", "domain", domain, "error", err)
				continue
			}
			s.domains[domain] = true
			added++
		}
	}

	s.logger.Info("Loaded domains from source", "source", source, "total", len(domains), "added", added)
	return nil
}

// addToBloomFilter adds a domain to the bloom filter
func (s *Service) addToBloomFilter(domain string) error {
	if s.bloomFilter == nil {
		return nil
	}

	// Calculate hash (matching C implementation)
	// static __always_inline int is_domain_blocked(const char *domain, int len) {
	//     __u32 hash = 0;
	//     for (int i = 0; i < len && i < 64; i++) {
	//         hash = hash * 31 + domain[i];
	//     }
	var hash uint32
	for i := 0; i < len(domain) && i < 64; i++ {
		hash = hash*31 + uint32(domain[i])
	}

	// Calculate index and bit
	// __u32 index = (hash % (131072 * 8)) / 8;
	// __u32 bit = hash % 8;
	index := (hash % (131072 * 8)) / 8
	bit := hash % 8

	// Read current byte
	var val uint8
	if err := s.bloomFilter.Lookup(&index, &val); err != nil {
		// If not found, val is already 0
		val = 0
	}

	// Set bit
	val |= (1 << bit)

	// Update map
	if err := s.bloomFilter.Update(&index, &val, ebpf.UpdateAny); err != nil {
		return fmt.Errorf("failed to update bloom filter map: %w", err)
	}

	return nil
}

// rebuildBloomFilter rebuilds the entire bloom filter
func (s *Service) rebuildBloomFilter() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.bloomFilter == nil {
		return
	}

	s.logger.Info("Rebuilding bloom filter", "count", len(s.domains))

	// Clear the map first
	var key uint32
	var zero uint8
	for i := uint32(0); i < 131072; i++ {
		key = i
		if err := s.bloomFilter.Update(&key, &zero, ebpf.UpdateAny); err != nil {
			s.logger.Error("Failed to clear bloom filter entry", "index", i, "error", err)
		}
	}

	// Re-add all domains
	for domain := range s.domains {
		var hash uint32
		for i := 0; i < len(domain) && i < 64; i++ {
			hash = hash*31 + uint32(domain[i])
		}

		index := (hash % (131072 * 8)) / 8
		bit := hash % 8

		var val uint8
		s.bloomFilter.Lookup(&index, &val)
		val |= (1 << bit)
		s.bloomFilter.Update(&index, &val, ebpf.UpdateAny)
	}

	s.logger.Info("Bloom filter rebuild complete")
}

// updateRoutine runs periodic updates
func (s *Service) updateRoutine() {
	if s.config.UpdateInterval == 0 {
		return
	}

	ticker := time.NewTicker(s.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.loadBlocklists(); err != nil {
				s.logger.Error("Failed to update blocklists", "error", err)
			}
			// Update settings
			s.config.UpdateInterval = 5 * time.Minute
			s.config.MaxDomains = 100000
			s.config.CleanupInterval = 1 * time.Hour
		case <-s.stopCh:
			return
		}
	}
}
