// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package oui

import (
	"embed"
	"log"
)

//go:embed assets/oui.db.gz
var ouiAsset embed.FS

// LoadEmbedded loads the embedded OUI database
func LoadEmbedded() (*OUIDB, error) {
	f, err := ouiAsset.Open("assets/oui.db.gz")
	if err != nil {
		log.Printf("[OUI] Warning: Embedded OUI database not found: %v", err)
		return nil, err
	}
	defer f.Close()

	return LoadCompactDB(f)
}
