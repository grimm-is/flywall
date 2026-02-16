// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package network

import (
	"bytes"
	"log"
	"os"
	"strings"

	"sync"

	"grimm.is/flywall/internal/network/oui_source/pkg/oui"
)

var (
	ouiDB *oui.OUIDB
	mu    sync.RWMutex
)

// InitOUI loads the OUI database.
// It prioritizes the local file at localPath if provided and valid.
// Falls back to the embedded asset.
func InitOUI(localPath string) {
	mu.Lock()
	defer mu.Unlock()

	var db *oui.OUIDB
	var err error
	loadedFrom := "embedded"

	// 1. Try local file if specified
	if localPath != "" {
		if data, err := os.ReadFile(localPath); err == nil {
			if fDB, err := oui.LoadCompactDB(bytes.NewReader(data)); err == nil {
				db = fDB
				loadedFrom = localPath
			}
		}
	}

	// 2. Fallback to embedded
	if db == nil {
		db, err = oui.LoadEmbedded()
		if err != nil {
			log.Printf("[OUI] Error loading embedded OUI DB: %v", err)
			return
		}
	}

	ouiDB = db
	log.Printf("[OUI] Loaded %d vendor prefixes from %s", len(db.Entries), loadedFrom)
}

// LoadFromBytes allows loading a DB from a byte slice (e.g. from state file)
func LoadFromBytes(data []byte) error {
	mu.Lock()
	defer mu.Unlock()

	db, err := oui.LoadCompactDB(bytes.NewReader(data))
	if err != nil {
		return err
	}
	ouiDB = db
	return nil
}

// LookupVendor returns the manufacturer for a MAC address.
// Returns "Random MAC" for locally administered (random) addresses.
func LookupVendor(mac string) string {
	mu.RLock()
	defer mu.RUnlock()

	if ouiDB == nil {
		return ""
	}

	// Normalize to raw hex "001122334455"
	// Remove all delimiters
	raw := strings.ReplaceAll(mac, ":", "")
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, ".", "")

	if len(raw) < 6 {
		return ""
	}

	raw = strings.ToUpper(raw)

	// Check for locally administered (random) MAC address
	// The second hex character indicates this: if bit 1 is set, it's locally administered
	// This means the second character is 2, 6, A, or E
	if len(raw) >= 2 {
		secondChar := raw[1]
		if secondChar == '2' || secondChar == '6' || secondChar == 'A' || secondChar == 'E' {
			return "Random MAC"
		}
	}

	// Longest Prefix Match Strategy

	// 1. Try MA-S / OUI-36 (36 bits = 9 hex chars)
	if len(raw) >= 9 {
		if entry, ok := ouiDB.Entries[raw[:9]]; ok {
			return entry.Manufacturer
		}
	}

	// 2. Try MA-M / OUI-28 (28 bits = 7 hex chars)
	if len(raw) >= 7 {
		if entry, ok := ouiDB.Entries[raw[:7]]; ok {
			return entry.Manufacturer
		}
	}

	// 3. Try OUI / MA-L (24 bits = 6 hex chars)
	if len(raw) >= 6 {
		if entry, ok := ouiDB.Entries[raw[:6]]; ok {
			return entry.Manufacturer
		}
	}

	return ""
}
