// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package firewall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewListManager_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewListManager(tmpDir, nil, "")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Verify default lists are loaded
	url, err := mgr.GetListURL("firehol_level1")
	if err != nil {
		t.Errorf("Expected firehol_level1 to be present, got error: %v", err)
	}
	if url == "" {
		t.Error("Expected URL for firehol_level1, got empty string")
	}

	// Verify unknown list
	_, err = mgr.GetListURL("non_existent_list")
	if err == nil {
		t.Error("Expected error for non_existent_list, got nil")
	}
}

func TestNewListManager_Override(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "custom.json")

	configContent := `{
		"lists": [
			{
				"name": "custom_list",
				"url": "http://example.com/list.txt",
				"description": "A custom list",
				"category": "test"
			},
			{
				"name": "firehol_level1",
				"url": "http://override.com/list.txt",
				"description": "Overridden list",
				"category": "override"
			}
		]
	}`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	mgr, err := NewListManager(tmpDir, nil, configFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Verify custom list
	url, err := mgr.GetListURL("custom_list")
	if err != nil {
		t.Errorf("Expected custom_list to be present: %v", err)
	}
	if url != "http://example.com/list.txt" {
		t.Errorf("Expected custom URL, got %s", url)
	}

	// Verify override
	url, err = mgr.GetListURL("firehol_level1")
	if err != nil {
		t.Errorf("Expected firehol_level1 to be present: %v", err)
	}
	if url != "http://override.com/list.txt" {
		t.Errorf("Expected overridden URL, got %s", url)
	}
}

func TestListManager_Caching(t *testing.T) {
	// Simple test to ensure cache directory is created
	tmpDir := t.TempDir()
	mgr, err := NewListManager(tmpDir, nil, "")
	if err != nil {
		t.Fatalf("NewListManager failed: %v", err)
	}

	// Calculate a cache key
	url := "http://example.com/testlist"
	key := mgr.generateCacheKey(url)

	// Save dummy cache
	data := []byte("1.2.3.4\n5.6.7.8")
	err = mgr.saveToCache(key, data, "etag123")
	if err != nil {
		t.Fatalf("saveToCache failed: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(tmpDir, key+".txt")); err != nil {
		t.Errorf("Cache data file not created")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, key+".meta")); err != nil {
		t.Errorf("Cache meta file not created")
	}

	// Load from cache
	ips, err := mgr.loadFromCache(key)
	if err != nil {
		t.Fatalf("loadFromCache failed: %v", err)
	}
	if len(ips) != 2 {
		t.Errorf("Expected 2 IPs, got %d", len(ips))
	}
}
