// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cloud

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
	"time"
)

// TelemetryLevel represents user-configured telemetry verbosity (slider)
// Higher levels include all data from lower levels
type TelemetryLevel int

const (
	// LevelMinimal: Heartbeat only (status, uptime) - ~4 MB/month
	LevelMinimal TelemetryLevel = 1
	// LevelDefault: + Health metrics (CPU, memory, latency) - ~13 MB/month
	LevelDefault TelemetryLevel = 2
	// LevelEnhanced: + Hourly summaries, top-N reports - ~15 MB/month
	LevelEnhanced TelemetryLevel = 3
	// LevelDetailed: + Per-interface stats, more frequent health - ~25 MB/month
	LevelDetailed TelemetryLevel = 4
)

// TelemetryConfig defines user-configurable telemetry settings
type TelemetryConfig struct {
	Level            TelemetryLevel `json:"level" hcl:"level"`
	HeartbeatSeconds int            `json:"heartbeat_seconds" hcl:"heartbeat_seconds"` // 30-300
	HealthMinutes    int            `json:"health_minutes" hcl:"health_minutes"`       // 1-60
	IncludeTopN      int            `json:"include_top_n" hcl:"include_top_n"`         // 0-10
}

// DefaultTelemetryConfig returns sensible defaults
func DefaultTelemetryConfig() TelemetryConfig {
	return TelemetryConfig{
		Level:            LevelDefault,
		HeartbeatSeconds: 30,
		HealthMinutes:    5,
		IncludeTopN:      5,
	}
}

// TelemetryTier defines the type of telemetry message
type TelemetryTier string

const (
	TierHeartbeat TelemetryTier = "heartbeat" // Always on
	TierHealth    TelemetryTier = "health"    // Level 2+
	TierSummary   TelemetryTier = "summary"   // Level 3+
	TierDebug     TelemetryTier = "debug"     // Support session only
)

// Heartbeat is sent every 30 seconds (50 bytes)
type Heartbeat struct {
	DeviceID      string    `json:"device_id"`
	Timestamp     time.Time `json:"ts"`
	Status        string    `json:"status"` // "healthy", "degraded", "error"
	ConfigVersion int64     `json:"config_version"`
	Uptime        int64     `json:"uptime_seconds"`
}

// HealthMetrics is sent every 5 minutes (80 bytes)
type HealthMetrics struct {
	DeviceID     string           `json:"device_id"`
	Timestamp    time.Time        `json:"ts"`
	CPUPercent   float32          `json:"cpu_pct"`
	MemPercent   float32          `json:"mem_pct"`
	DiskPercent  float32          `json:"disk_pct"`
	WANLatencyMs int32            `json:"wan_latency_ms"`
	Interfaces   []InterfaceStats `json:"ifaces,omitempty"`
}

// InterfaceStats for each network interface
type InterfaceStats struct {
	Name     string `json:"name"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	Errors   uint32 `json:"errors"`
}

// SummaryStats is sent hourly (opt-in, ~300 bytes)
type SummaryStats struct {
	DeviceID        string    `json:"device_id"`
	Timestamp       time.Time `json:"ts"`
	PeriodStart     time.Time `json:"period_start"`
	TotalBytesIn    uint64    `json:"total_bytes_in"`
	TotalBytesOut   uint64    `json:"total_bytes_out"`
	BlockedCount    uint32    `json:"blocked_count"`
	TopBlocked      []string  `json:"top_blocked,omitempty"`      // Top 5 blocked IPs (hashed)
	TopDestinations []string  `json:"top_destinations,omitempty"` // Top 5 destinations (hashed)
}

// SecurityAlert is sent on-event (~250 bytes)
type SecurityAlert struct {
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"ts"`
	AlertID   string    `json:"alert_id"`
	Severity  string    `json:"severity"` // "info", "warning", "critical"
	Type      string    `json:"type"`     // "threshold", "policy", "intrusion"
	Message   string    `json:"message"`
	Context   string    `json:"context,omitempty"` // Minimal context, no PII
}

// TelemetryBatch bundles multiple telemetry items for efficient transmission
type TelemetryBatch struct {
	DeviceID   string           `json:"device_id"`
	BatchID    string           `json:"batch_id"`
	Timestamp  time.Time        `json:"ts"`
	BucketID   int64            `json:"bucket_id,omitempty"` // Blind Index: Time bucket
	Heartbeats []Heartbeat      `json:"heartbeats,omitempty"`
	Health     []HealthMetrics  `json:"health,omitempty"`
	Summaries  []SummaryStats   `json:"summaries,omitempty"`
	Alerts     []SecurityAlert  `json:"alerts,omitempty"`
	Tags       map[string]int16 `json:"tags,omitempty"` // Blind Index: Search tags
}

// Prepare populates blind index fields (BucketID and Tags) for the batch.
// masterKey is the user's vault master secret, used to derive the monthly salt.
func (b *TelemetryBatch) Prepare(masterKey []byte) {
	if b.Timestamp.IsZero() {
		b.Timestamp = time.Now()
	}

	// 1. Set Bucket ID (Hourly)
	b.BucketID = BucketTimestamp(b.Timestamp)

	// 2. Generate Search Tags (Blind Index)
	salt := GenerateBlindIndexSalt(masterKey, b.Timestamp)
	b.Tags = make(map[string]int16)

	// Example tags from alerts
	for _, alert := range b.Alerts {
		if alert.Type != "" {
			b.Tags["type:"+alert.Type] = CalculateBlindTag(salt, "type:"+alert.Type)
		}
		if alert.Severity != "" {
			b.Tags["severity:"+alert.Severity] = CalculateBlindTag(salt, "severity:"+alert.Severity)
		}
	}

	// Example tags from summaries (Top-N destinations)
	for _, dest := range b.Summaries {
		for _, ip := range dest.TopDestinations {
			b.Tags["dst_ip:"+ip] = CalculateBlindTag(salt, "dst_ip:"+ip)
		}
	}
}

// MetadataPayload is typically sent as a separate unencrypted event
// to allow the server to index the data without reading the payload.
type MetadataPayload struct {
	BucketID int64            `json:"bucket_id"`
	Tags     map[string]int16 `json:"tags"`
}

// Marshal returns JSON bytes for transmission
func (b *TelemetryBatch) Marshal() ([]byte, error) {
	return json.Marshal(b)
}

// Encrypt payload using AES-GCM.
// Returns a new TelemetryBatch with the payload replaced by an encrypted blob (conceptually),
// but since the proto definition separates "TelemetryBatch" (the struct) from the "EncryptedPayload" message,
// we might need to adjust how this is called.
//
// However, adhering to the design where the "Payload" field in the gRPC message can be encrypted,
// or the TelemetryBatch itself contains encrypted fields.
//
// Looking at `device.proto`, TelemetryBatch has `repeated TelemetryEvent events`.
// TelemetryEvent has `bytes payload`.
//
// The Zero-Knowledge design states: "Agent encrypts telemetry batches".
// This implies the *entire* batch or individual events are encrypted.
//
// If we encrypt individual event payloads, the metadata (timestamp, type) remains visible to Cloud.
// This is GOOD for alerting/metrics but bad if metadata leaks PII.
// Design says: "Cloud stores encrypted_payload and key_id".
// And "Telemetry Encryption... Agent encrypts telemetry batches".
//
// Let's implement a helper to encrypt a slice of bytes, which the client will use.
// But wait, this file is `packet` struct definitions.
// Let's stick to the plan: "Implement Encrypt(key []byte) method (AES-GCM)"
//
// We'll add a helper function `EncryptPayload` here.

// EncryptPayload encrypts data using AES-GCM with the given key.
// It returns nonce + ciphertext.
func EncryptPayload(key []byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}
