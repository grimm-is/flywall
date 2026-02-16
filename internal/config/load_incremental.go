// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// IncrementalLoader loads configuration sections incrementally with caching
type IncrementalLoader struct {
	cache      *ConfigCache
	validators map[string]SectionValidator
	mu         sync.RWMutex
	loadTimeout time.Duration
}

// ConfigCache caches configuration sections with hash-based change detection
type ConfigCache struct {
	sections map[string]*CachedSection
	mu       sync.RWMutex
}

// CachedSection represents a cached configuration section
type CachedSection struct {
	Data      interface{}
	Hash      string
	Timestamp time.Time
	Valid     bool
	Dependencies []string // Sections this depends on
}

// SectionValidator validates a specific configuration section
type SectionValidator interface {
	Validate(data interface{}) error
	Dependencies() []string
}

// NewIncrementalLoader creates a new incremental configuration loader
func NewIncrementalLoader() *IncrementalLoader {
	return &IncrementalLoader{
		cache:      NewConfigCache(),
		validators: make(map[string]SectionValidator),
		loadTimeout: 30 * time.Second,
	}
}

// NewConfigCache creates a new configuration cache
func NewConfigCache() *ConfigCache {
	return &ConfigCache{
		sections: make(map[string]*CachedSection),
	}
}

// LoadSection loads a single configuration section with caching
func (il *IncrementalLoader) LoadSection(ctx context.Context, name string, data []byte) error {
	il.mu.Lock()
	defer il.mu.Unlock()

	// Calculate hash of new data
	newHash := calculateHash(data)

	// Check cache
	if cached, exists := il.cache.Get(name); exists {
		if cached.Hash == newHash && cached.Valid {
			// Data hasn't changed and is still valid
			return nil
		}
	}

	// Parse section data
	var sectionData interface{}
	switch name {
	case "interfaces":
		var interfaces []Interface
		if err := json.Unmarshal(data, &interfaces); err != nil {
			return fmt.Errorf("failed to parse interfaces: %w", err)
		}
		sectionData = interfaces
	case "zones":
		var zones []Zone
		if err := json.Unmarshal(data, &zones); err != nil {
			return fmt.Errorf("failed to parse zones: %w", err)
		}
		sectionData = zones
	case "policies":
		var policies []Policy
		if err := json.Unmarshal(data, &policies); err != nil {
			return fmt.Errorf("failed to parse policies: %w", err)
		}
		sectionData = policies
	default:
		return fmt.Errorf("unknown section: %s", name)
	}

	// Validate section
	if validator, exists := il.getValidator(name); exists {
		if err := validator.Validate(sectionData); err != nil {
			return fmt.Errorf("validation failed for section %s: %w", name, err)
		}
	}

	// Update cache
	il.cache.Put(name, &CachedSection{
		Data:      sectionData,
		Hash:      newHash,
		Timestamp: time.Now(),
		Valid:     true,
	})

	return nil
}

// LoadConfigIncrementally loads a complete configuration incrementally
func (il *IncrementalLoader) LoadConfigIncrementally(ctx context.Context, config *Config) error {
	// Serialize config sections
	sections := make(map[string][]byte)
	
	if data, err := json.Marshal(config.Interfaces); err == nil {
		sections["interfaces"] = data
	}
	if data, err := json.Marshal(config.Zones); err == nil {
		sections["zones"] = data
	}
	if data, err := json.Marshal(config.Policies); err == nil {
		sections["policies"] = data
	}

	// Load sections in dependency order
	loadOrder := []string{"interfaces", "zones", "policies"}
	
	for _, sectionName := range loadOrder {
		if data, exists := sections[sectionName]; exists {
			if err := il.LoadSection(ctx, sectionName, data); err != nil {
				return fmt.Errorf("failed to load section %s: %w", sectionName, err)
			}
		}
	}

	return nil
}

// GetCachedSection retrieves a cached section
func (il *IncrementalLoader) GetCachedSection(name string) (*CachedSection, bool) {
	return il.cache.Get(name)
}

// InvalidateSection marks a section as invalid, forcing reload
func (il *IncrementalLoader) InvalidateSection(name string) {
	il.cache.Invalidate(name)
	
	// Also invalidate dependent sections
	if cached, exists := il.cache.Get(name); exists {
		for _, dep := range cached.Dependencies {
			il.cache.Invalidate(dep)
		}
	}
}

// RegisterValidator registers a validator for a section
func (il *IncrementalLoader) RegisterValidator(name string, validator SectionValidator) {
	il.mu.Lock()
	defer il.mu.Unlock()
	il.validators[name] = validator
}

// getValidator retrieves a validator for a section
func (il *IncrementalLoader) getValidator(name string) (SectionValidator, bool) {
	il.mu.RLock()
	defer il.mu.RUnlock()
	validator, exists := il.validators[name]
	return validator, exists
}

// Get retrieves a section from cache
func (cc *ConfigCache) Get(name string) (*CachedSection, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	section, exists := cc.sections[name]
	return section, exists
}

// Put stores a section in cache
func (cc *ConfigCache) Put(name string, section *CachedSection) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.sections[name] = section
}

// Invalidate marks a section as invalid
func (cc *ConfigCache) Invalidate(name string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if section, exists := cc.sections[name]; exists {
		section.Valid = false
	}
}

// Clear clears all cached sections
func (cc *ConfigCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.sections = make(map[string]*CachedSection)
}

// GetStats returns cache statistics
func (cc *ConfigCache) GetStats() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	stats := make(map[string]interface{})
	stats["total_sections"] = len(cc.sections)
	
	validCount := 0
	for _, section := range cc.sections {
		if section.Valid {
			validCount++
		}
	}
	stats["valid_sections"] = validCount
	stats["invalid_sections"] = len(cc.sections) - validCount
	
	return stats
}

// calculateHash calculates SHA256 hash of data
func calculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// InterfacesValidator validates interfaces section
type InterfacesValidator struct{}

func (v *InterfacesValidator) Validate(data interface{}) error {
	interfaces, ok := data.([]Interface)
	if !ok {
		return fmt.Errorf("invalid data type for interfaces")
	}
	
	// Basic validation
	for _, iface := range interfaces {
		if iface.Name == "" {
			return fmt.Errorf("interface name is required")
		}
	}
	
	return nil
}

func (v *InterfacesValidator) Dependencies() []string {
	return []string{}
}

// ZonesValidator validates zones section
type ZonesValidator struct{}

func (v *ZonesValidator) Validate(data interface{}) error {
	zones, ok := data.([]Zone)
	if !ok {
		return fmt.Errorf("invalid data type for zones")
	}
	
	// Basic validation
	for _, zone := range zones {
		if zone.Name == "" {
			return fmt.Errorf("zone name is required")
		}
	}
	
	return nil
}

func (v *ZonesValidator) Dependencies() []string {
	return []string{"interfaces"}
}

// PoliciesValidator validates policies section
type PoliciesValidator struct{}

func (v *PoliciesValidator) Validate(data interface{}) error {
	policies, ok := data.([]Policy)
	if !ok {
		return fmt.Errorf("invalid data type for policies")
	}
	
	// Basic validation
	for _, policy := range policies {
		if policy.From == "" {
			return fmt.Errorf("policy 'from' zone is required")
		}
		if policy.To == "" {
			return fmt.Errorf("policy 'to' zone is required")
		}
	}
	
	return nil
}

func (v *PoliciesValidator) Dependencies() []string {
	return []string{"interfaces", "zones"}
}
