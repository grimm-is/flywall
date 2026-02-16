// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package state

import (
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestSchemaMigration_Backfill(t *testing.T) {
	// 1. Create a legacy database (manually)
	tmpFile, err := os.CreateTemp("", "flywall-legacy-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Initialize legacy schema (no hash column)
	_, err = db.Exec(`
		CREATE TABLE buckets (name TEXT PRIMARY KEY);
		CREATE TABLE entries (
			bucket TEXT,
			key TEXT,
			value BLOB,
			version INTEGER,
			updated_at DATETIME,
			expires_at DATETIME,
			PRIMARY KEY (bucket, key)
		);
		CREATE TABLE changes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket TEXT,
			key TEXT,
			value BLOB,
			change_type TEXT,
			version INTEGER,
			timestamp DATETIME
			-- NO HASH COLUMN
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert legacy data
	// Change 1
	_, err = db.Exec(`INSERT INTO changes (bucket, key, value, change_type, version, timestamp) VALUES ('b', 'k1', 'v1', 'insert', 1, ?)`, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Change 2
	_, err = db.Exec(`INSERT INTO changes (bucket, key, value, change_type, version, timestamp) VALUES ('b', 'k2', 'v2', 'insert', 2, ?)`, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Close raw DB connection
	db.Close()

	// 2. Open with SQLiteStore (triggers migration)
	// logger := logging.New(logging.DefaultConfig()) // Unused
	// Opts usually takes a DSN string directly or via struct?
	// Checking code_item... Options is actually just a string alias or struct with specific fields?
	// Ah, NewSQLiteStore takes (opts Options). verify definition.

	// Assuming Options is string based on NewSQLiteStore usage in previous view_file (line 72 of output 1493: DefaultOptions(":memory:"))
	// Wait, DefaultOptions(":memory:") returns Options.

	opts := DefaultOptions(dbPath)
	store, err := NewSQLiteStore(opts)
	if err != nil {
		t.Fatalf("Failed to open store (migration should run): %v", err)
	}
	defer store.Close()

	// 3. Verify Migration
	// Check if hash column exists and is populated
	changes, err := store.GetChangesSince(0)
	if err != nil {
		t.Fatal(err)
	}

	if len(changes) != 2 {
		t.Fatalf("Expected 2 changes, got %d", len(changes))
	}

	c1 := changes[0]
	c2 := changes[1]

	if c1.Hash == "" {
		t.Error("Change 1 hash is empty (backfill failed)")
	}
	if c2.Hash == "" {
		t.Error("Change 2 hash is empty (backfill failed)")
	}

	// Verify chain integrity
	// Re-compute expected hashes
	expectedH1 := store.computeHash("", c1)
	if c1.Hash != expectedH1 {
		t.Errorf("Change 1 hash mismatch. Got %s, want %s", c1.Hash, expectedH1)
	}

	expectedH2 := store.computeHash(c1.Hash, c2)
	if c2.Hash != expectedH2 {
		t.Errorf("Change 2 hash mismatch. Got %s, want %s", c2.Hash, expectedH2)
	}
}
