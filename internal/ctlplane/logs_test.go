// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ctlplane

import (
	"os"
	"testing"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/services/dns/querylog"
)

func TestGetDNSLogs(t *testing.T) {
	// Create temporary query log DB
	tmpFile, err := os.CreateTemp("", "querylog_test.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	store, err := querylog.Open(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Seed some data
	entry1 := querylog.Entry{
		Timestamp:  time.Now().Add(-1 * time.Minute),
		ClientIP:   "192.168.1.10",
		Domain:     "example.com",
		Type:       "A",
		RCode:      "NOERROR",
		Upstream:   "8.8.8.8",
		DurationMs: 25,
	}
	if err := store.RecordEntry(entry1); err != nil {
		t.Fatal(err)
	}

	entry2 := querylog.Entry{
		Timestamp:  time.Now(),
		ClientIP:   "192.168.1.11",
		Domain:     "malware.site",
		Type:       "A",
		RCode:      "NXDOMAIN",
		Blocked:    true,
		BlockList:  "BadSites",
		DurationMs: 1,
	}
	if err := store.RecordEntry(entry2); err != nil {
		t.Fatal(err)
	}

	// Create Server
	server := NewServer(&config.Config{}, "", nil)
	server.SetQueryLogStore(store)

	// Test GetLogs with DNS source
	args := &GetLogsArgs{
		Source: "dns",
		Limit:  10,
	}

	// We need to call getDNSLogs directly or via GetLogs
	// Since getDNSLogs is unexported but we are in ctlplane package, we can call it?
	// Wait, getDNSLogs is receiving *Server. Yes methods are accessible.
	// But GetLogs is the public API.

	var reply GetLogsReply
	if err := server.GetLogs(args, &reply); err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}

	if len(reply.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(reply.Entries))
	}

	// Verify order (newest first usually? Store.GetRecentLogs returns newest first)
	// Entry2 is newer.
	if len(reply.Entries) > 0 {
		e1 := reply.Entries[0]
		if e1.Extra["DOMAIN"] != "malware.site" {
			t.Errorf("Expected first entry to be malware.site, got %s", e1.Extra["DOMAIN"])
		}
		if e1.Extra["BLOCKED"] != "true" {
			t.Error("Expected blocked flag on malware site")
		}
	}
}
