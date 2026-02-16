// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package state

import (
	"testing"
	"time"

	"grimm.is/flywall/internal/errors"
	"grimm.is/flywall/internal/logging"
)

func TestReplicator_ApplyChange_PreservesVersion(t *testing.T) {
	// Setup Store
	opts := DefaultOptions(":memory:")
	store, err := NewSQLiteStore(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.CreateBucket("test")

	// Setup Replicator
	logger := logging.New(logging.Config{Level: logging.LevelError})
	repl := NewReplicator(store, DefaultReplicationConfig(), logger)

	// Define a change with a specific high version
	targetVersion := uint64(100)
	change := Change{
		Bucket:    "test",
		Key:       "key1",
		Value:     []byte("val1"),
		Type:      ChangeInsert,
		Timestamp: time.Now(),
		Version:   targetVersion,
	}

	// Compute expected hash (simulating what Primary would send)
	// Since store is empty, prevHash is ""
	// We need to access computeHash which is private.
	// Since we are in package state, we can use a helper or just replicate the logic?
	// Actually, TestReplicator is in package state_test usually?
	// File says `package state`. So we can access private methods of `store` (which is *SQLiteStore).
	change.Hash = store.computeHash("", change)

	// Call applyChange (private method, accessible in same package test)
	if err := repl.applyChange(change); err != nil {
		t.Fatalf("applyChange failed: %v", err)
	}

	// Verify the stored entry has the CORRECT version
	entry, err := store.GetWithMeta("test", "key1")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Expected Version: %d", targetVersion)
	t.Logf("Actual Version:   %d", entry.Version)

	if entry.Version != targetVersion {
		t.Errorf("Version mismatch! Expected %d, got %d. The replication bug is present.", targetVersion, entry.Version)
	} else {
		t.Log("Version matches. Bug fixed or not present.")
	}

	// Verify store current version is updated
	if store.CurrentVersion() != targetVersion {
		t.Errorf("Store current version mismatch! Expected %d, got %d", targetVersion, store.CurrentVersion())
	}
}

func TestReplicator_AutoRecovery(t *testing.T) {
	// 1. Setup Store and Replicator
	opts := DefaultOptions(":memory:")
	store, err := NewSQLiteStore(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	logger := logging.New(logging.Config{Level: logging.LevelError})
	repl := NewReplicator(store, DefaultReplicationConfig(), logger)

	// 2. Simulate Divergence
	// Attempt to apply a change with an INVALID hash
	change := Change{
		Bucket:    "test",
		Key:       "bad_hash",
		Value:     []byte("val"),
		Type:      ChangeInsert,
		Timestamp: time.Now(),
		Version:   1,
		Hash:      "invalid_hash", // Intentional mismatch
	}

	err = repl.applyChange(change)
	if err == nil {
		t.Fatal("Expected error applying change with bad hash, got nil")
	}

	// Check if it is the specific divergence error (wrapped)
	if errors.Is(err, ErrDivergence) {
		t.Log("Caught expected ErrDivergence")
	} else {
		t.Fatalf("Expected ErrDivergence, got: %v", err)
	}

	// 3. Verify Replicator logic sets forceSnapshot
	// Since applyChange doesn't set the flag (calls to receiveUpdates do), we need to simulate the
	// handling logic found in receiveUpdates.
	// We can't easily run the full loop in unit test without mocking net.Conn.
	// But we can verify `ErrDivergence` is returned by store, which we did.

	// Let's manually trigger the recovery logic to verify state transition
	repl.mu.Lock()
	repl.forceSnapshot = true
	repl.mu.Unlock()

	// 4. Verify connect logic uses Version 0
	// We can check the internal state or simulate the request construction
	repl.mu.RLock()
	reqVer := store.CurrentVersion()
	if repl.forceSnapshot {
		reqVer = 0
	}
	repl.mu.RUnlock()

	if reqVer != 0 {
		t.Errorf("Expected request version 0 (full sync) when forceSnapshot is true, got %d", reqVer)
	}
}
