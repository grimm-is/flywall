// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"grimm.is/flywall/internal/install"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/clock"
	"grimm.is/flywall/internal/logging"
)

//go:embed iplists.json
var defaultListsJSON []byte

const (
	// DefaultCacheTTL is the default cache duration for IP lists.
	DefaultCacheTTL = 24 * time.Hour
)

// ManagedList defines a remote IP list configuration.
type ManagedList struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// ManagedListRegistry holds the map of available lists.
type ManagedListRegistry struct {
	Lists map[string]ManagedList `json:"lists"`
}

// ListManager handles downloading and managing external IP lists.
type ListManager struct {
	cacheDir string
	logger   *logging.Logger
	registry ManagedListRegistry
	mu       sync.RWMutex
}

// NewListManager creates a new ListManager.
// It loads default lists from embedded JSON and optionally overrides from a file.
func NewListManager(cacheDir string, logger *logging.Logger, configFile string) (*ListManager, error) {
	if logger == nil {
		logger = logging.New(logging.DefaultConfig())
	}
	if cacheDir == "" {
		cacheDir = filepath.Join(install.GetCacheDir(), "iplists")
	}

	mgr := &ListManager{
		cacheDir: cacheDir,
		logger:   logger,
		registry: ManagedListRegistry{Lists: make(map[string]ManagedList)},
	}

	// 1. Load defaults
	if err := mgr.loadFromBytes(defaultListsJSON); err != nil {
		return nil, fmt.Errorf("failed to load default lists: %w", err)
	}

	// 2. Load overrides/additions if file provided
	if configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			data, err := os.ReadFile(configFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read list config %s: %w", configFile, err)
			}
			if err := mgr.loadFromBytes(data); err != nil {
				return nil, fmt.Errorf("failed to parse list config %s: %w", configFile, err)
			}
		}
	}

	return mgr, nil
}

func (m *ListManager) loadFromBytes(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var temp struct {
		Lists []ManagedList `json:"lists"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	for _, l := range temp.Lists {
		m.registry.Lists[l.Name] = l
	}
	return nil
}

// GetListURL returns the URL for a named list.
func (m *ListManager) GetListURL(name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if list, ok := m.registry.Lists[name]; ok {
		return list.URL, nil
	}
	return "", fmt.Errorf("managed list not found: %s", name)
}

// DownloadList downloads a managed list by name.
func (m *ListManager) DownloadList(name string) ([]string, error) {
	url, err := m.GetListURL(name)
	if err != nil {
		return nil, err
	}
	return m.DownloadFromURL(url)
}

// DownloadFromURL downloads an IP list from any URL with caching support.
func (m *ListManager) DownloadFromURL(url string) ([]string, error) {
	// Generate cache key from URL
	cacheKey := m.generateCacheKey(url)

	// Try to load from cache first
	if ips, err := m.loadFromCache(cacheKey); err == nil {
		return ips, nil
	}

	// Download fresh data
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download %s: status %d", url, resp.StatusCode)
	}

	var reader io.Reader = resp.Body

	// Handle gzip-compressed responses
	if strings.HasSuffix(url, ".gz") || resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Limit reader to 10MB
	limitReader := io.LimitReader(reader, 10*1024*1024)

	// Read into memory
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the list
	ips, err := ParseIPList(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse IP list: %w", err)
	}

	// Save to cache
	if err := m.saveToCache(cacheKey, data, resp.Header.Get("ETag")); err != nil {
		m.logger.Warn("Failed to cache list", "url", url, "error", err)
	}

	return ips, nil
}

// ClearCache removes all cached files.
func (m *ListManager) ClearCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cacheDir == "" {
		return nil
	}
	return os.RemoveAll(m.cacheDir)
}

// GetCacheInfo returns information about cached lists.
func (m *ListManager) GetCacheInfo() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cacheDir == "" {
		return map[string]interface{}{"cached_lists": 0}, nil
	}

	files, err := os.ReadDir(m.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{"cached_lists": 0}, nil
		}
		return nil, err
	}

	cachedLists := 0
	totalSize := int64(0)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".txt") {
			cachedLists++
			info, err := file.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
	}

	return map[string]interface{}{
		"cached_lists": cachedLists,
		"total_size":   totalSize,
		"cache_dir":    m.cacheDir,
	}, nil
}

// Helper methods (cache key generation, file I/O) mirroring previous implementation

func (m *ListManager) generateCacheKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}

func (m *ListManager) saveToCache(cacheKey string, data []byte, etag string) error {
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return err
	}
	dataPath := filepath.Join(m.cacheDir, cacheKey+".txt")
	if err := os.WriteFile(dataPath, data, 0644); err != nil {
		return err
	}

	metadata := map[string]interface{}{
		"cached_at": clock.Now().Unix(),
		"etag":      etag,
		"size":      len(data),
		"checksum":  m.calculateChecksum(data),
	}
	metadataData, _ := json.Marshal(metadata)
	return os.WriteFile(filepath.Join(m.cacheDir, cacheKey+".meta"), metadataData, 0644)
}

func (m *ListManager) loadFromCache(cacheKey string) ([]string, error) {
	dataPath := filepath.Join(m.cacheDir, cacheKey+".txt")
	metadataPath := filepath.Join(m.cacheDir, cacheKey+".meta")

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache miss")
	}

	// Load metadata validation (simplified for brevity)
	metaData, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal(metaData, &metadata); err != nil {
		return nil, err
	}

	// Check expiry
	if cachedAt, ok := metadata["cached_at"].(float64); ok {
		if time.Since(time.Unix(int64(cachedAt), 0)) > DefaultCacheTTL {
			return nil, fmt.Errorf("cache expired")
		}
	}

	data, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, err
	}
	return ParseIPList(bytes.NewReader(data))
}

func (m *ListManager) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Needed by ParseIPList util usage, assuming ParseIPList is in this package or util
// Re-implementing small utility if it was in firehol.go locally.
// Actually ParseIPList was extracted from firehol.go content I viewed earlier?
// Wait, ParseIPList was likely in firehol.go. I need to ensure it's preserved.
