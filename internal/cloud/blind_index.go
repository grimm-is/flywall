// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cloud

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"time"
)

// CalculateBlindTag computes a deterministic, truncated hash for a value.
// It uses HMAC-SHA256 with a salt, and returns the first 2 bytes (int16).
// This provides searchability while preserving privacy via high collision rate (truncation).
func CalculateBlindTag(salt []byte, value string) int16 {
	h := hmac.New(sha256.New, salt)
	h.Write([]byte(value))
	sum := h.Sum(nil)

	// Take first 2 bytes
	// We use BigEndian to be consistent across platforms/languages
	tag := binary.BigEndian.Uint16(sum[:2])

	// Return as int16 (signed), matching Postgres SMALLINT
	return int16(tag)
}

// BucketTimestamp truncates a timestamp to a coarse bucket ID (e.g. hourly).
// This allows range queries on encrypted data without revealing exact event times.
// Current strategy: Unix timestamp / 3600 (Hourly buckets)
func BucketTimestamp(ts time.Time) int64 {
	return ts.Unix() / 3600
}

// GenerateBlindIndexSalt derives a time-based salt (e.g. monthly).
// This ensures forward secrecy: a compromise of the key only reveals logs for that month.
// salt = HMAC(masterKey, "2026-01")
func GenerateBlindIndexSalt(masterKey []byte, ts time.Time) []byte {
	monthStr := ts.Format("2006-01")
	h := hmac.New(sha256.New, masterKey)
	h.Write([]byte(monthStr))
	return h.Sum(nil)
}
