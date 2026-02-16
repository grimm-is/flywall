// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package network

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"grimm.is/flywall/internal/network/oui_source/pkg/oui"
)

const (
	// Public GitHub raw URL for the OUI database
	GithubOUISource = "https://raw.githubusercontent.com/grimm-is/flywall-oui/main/pkg/oui/assets/oui.db.gz"
)

// UpdateOUI fetches the latest OUI database from GitHub and saves it to the specified path.
// It performs verification before replacing the existing file.
func UpdateOUI(ctx context.Context, destPath string) error {
	// 1. Download to temporary file
	tempFile := destPath + ".tmp"
	defer os.Remove(tempFile) // Cleanup if we fail

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", GithubOUISource, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download OUI db: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	out, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	// Stream valid GZIP content?
	// We'll read into file, then verify later, or tee reader?
	// Let's just write to disk first to avoid memory issues if it gets huge.
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	out.Close() // Close specifically to ensure flush

	// 2. Verify integrity
	f, err := os.Open(tempFile)
	if err != nil {
		return fmt.Errorf("failed to open downloaded file for verification: %w", err)
	}
	defer f.Close()

	// Try to load header/basic structure using library
	// This ensures it's a valid GZIP and Gob stream
	db, err := oui.LoadCompactDB(f)
	if err != nil {
		return fmt.Errorf("downloaded file is invalid or corrupt: %w", err)
	}

	if len(db.Entries) < 1000 {
		return fmt.Errorf("downloaded db seems too small (%d entries), rejecting", len(db.Entries))
	}
	f.Close()

	// 3. Atomic replace
	if err := os.Rename(tempFile, destPath); err != nil {
		return fmt.Errorf("failed to move temp file to destination: %w", err)
	}

	// 4. Reload in-memory
	// We read the file we just wrote
	data, err := os.ReadFile(destPath)
	if err != nil {
		return fmt.Errorf("failed to read back new db: %w", err)
	}

	if err := LoadFromBytes(data); err != nil {
		return fmt.Errorf("failed to reload db into memory: %w", err)
	}

	return nil
}
