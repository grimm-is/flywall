// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package orca

import (
	"io"
	"strings"
)

// RawModeWriter acts as a pass-through writer that injects \r before \n
// This is necessary when the terminal is in raw mode.
type RawModeWriter struct {
	Target io.Writer
}

func (w *RawModeWriter) Write(p []byte) (n int, err error) {
	// Simple implementation: replace \n with \r\n
	// Note: efficient buffering would be better but for logs this is fine.
	out := strings.ReplaceAll(string(p), "\n", "\r\n")
	// If the string already has \r\n, ReplaceAll would make it \r\r\n.
	// A more robust check:
	out = strings.ReplaceAll(string(p), "\r\n", "\n") // normalize
	out = strings.ReplaceAll(out, "\n", "\r\n")       // apply

	_, err = w.Target.Write([]byte(out))
	// Return len(p) so callers are satisfied, even though we wrote more
	if err == nil {
		return len(p), nil
	}
	return 0, err
}
