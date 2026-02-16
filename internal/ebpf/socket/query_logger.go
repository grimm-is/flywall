// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"
)

// QueryLogger handles DNS query logging
type QueryLogger struct {
	// Configuration
	config *QueryLoggerConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Log file
	logFile *os.File
	writer  *bufio.Writer

	// Statistics
	stats *QueryLoggerStats

	// Buffer for batching
	eventBuffer []*types.DNSQueryEvent
	bufferMutex sync.Mutex

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// QueryLoggerConfig holds configuration for query logging
type QueryLoggerConfig struct {
	// Log file settings
	LogFilePath     string `hcl:"log_file_path,optional"`
	MaxFileSize     int64  `hcl:"max_file_size,optional"` // in bytes
	MaxBackups      int    `hcl:"max_backups,optional"`
	CompressBackups bool   `hcl:"compress_backups,optional"`

	// Buffer settings
	BufferSize    int           `hcl:"buffer_size,optional"`
	FlushInterval time.Duration `hcl:"flush_interval,optional"`

	// Filter settings
	LogQueries     bool `hcl:"log_queries,optional"`
	LogResponses   bool `hcl:"log_responses,optional"`
	LogPrivateIPs  bool `hcl:"log_private_ips,optional"`
	LogBlockedOnly bool `hcl:"log_blocked_only,optional"`

	// Format settings
	LogFormat           string `hcl:"log_format,optional"` // json, text
	IncludeTimestamp    bool   `hcl:"include_timestamp,optional"`
	IncludePID          bool   `hcl:"include_pid,optional"`
	IncludeResponseTime bool   `hcl:"include_response_time,optional"`
}

// QueryLoggerStats holds statistics for query logging
type QueryLoggerStats struct {
	QueriesLogged   uint64    `json:"queries_logged"`
	ResponsesLogged uint64    `json:"responses_logged"`
	BytesWritten    uint64    `json:"bytes_written"`
	FilesRotated    uint64    `json:"files_rotated"`
	BufferFlushes   uint64    `json:"buffer_flushes"`
	Errors          uint64    `json:"errors"`
	LastFlush       time.Time `json:"last_flush"`
	LastRotation    time.Time `json:"last_rotation"`
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp    time.Time `json:"timestamp,omitempty"`
	Type         string    `json:"type"` // query, response
	PID          uint32    `json:"pid,omitempty"`
	TID          uint32    `json:"tid,omitempty"`
	SourceIP     string    `json:"source_ip"`
	SourcePort   uint16    `json:"source_port"`
	DestIP       string    `json:"dest_ip"`
	DestPort     uint16    `json:"dest_port"`
	QueryID      uint16    `json:"query_id"`
	Domain       string    `json:"domain"`
	QueryType    uint16    `json:"query_type,omitempty"`
	QueryClass   uint16    `json:"query_class,omitempty"`
	ResponseCode uint8     `json:"response_code,omitempty"`
	AnswerCount  uint16    `json:"answer_count,omitempty"`
	ResponseTime string    `json:"response_time,omitempty"`
	PacketSize   uint16    `json:"packet_size"`
	Blocked      bool      `json:"blocked,omitempty"`
	Reason       string    `json:"reason,omitempty"`
}

// DefaultQueryLoggerConfig returns default configuration
func DefaultQueryLoggerConfig() *QueryLoggerConfig {
	return &QueryLoggerConfig{
		LogFilePath:         "/var/log/flywall/dns_queries.log",
		MaxFileSize:         100 * 1024 * 1024, // 100MB
		MaxBackups:          5,
		CompressBackups:     true,
		BufferSize:          1000,
		FlushInterval:       5 * time.Second,
		LogQueries:          true,
		LogResponses:        true,
		LogPrivateIPs:       false,
		LogBlockedOnly:      false,
		LogFormat:           "json",
		IncludeTimestamp:    true,
		IncludePID:          true,
		IncludeResponseTime: true,
	}
}

// NewQueryLogger creates a new query logger
func NewQueryLogger(logger *logging.Logger, config *QueryLoggerConfig) *QueryLogger {
	if config == nil {
		config = DefaultQueryLoggerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	ql := &QueryLogger{
		config:      config,
		stats:       &QueryLoggerStats{},
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		eventBuffer: make([]*types.DNSQueryEvent, 0, config.BufferSize),
	}

	return ql
}

// Start starts the query logger
func (ql *QueryLogger) Start() error {
	ql.mutex.Lock()
	defer ql.mutex.Unlock()

	if ql.config.LogFilePath == "" {
		ql.logger.Info("Query logging disabled (no log file path)")
		return nil
	}

	ql.logger.Info("Starting DNS query logger",
		"log_file", ql.config.LogFilePath,
		"format", ql.config.LogFormat)

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(ql.config.LogFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	if err := ql.openLogFile(); err != nil {
		return err
	}

	// Start flush goroutine
	go ql.flushWorker()

	ql.enabled = true
	ql.logger.Info("DNS query logger started")

	return nil
}

// Stop stops the query logger
func (ql *QueryLogger) Stop() {
	ql.mutex.Lock()
	defer ql.mutex.Unlock()

	if !ql.enabled {
		return
	}

	ql.logger.Info("Stopping DNS query logger")

	// Cancel context
	ql.cancel()

	// Flush remaining buffer
	ql.flushBuffer()

	// Close log file
	if ql.writer != nil {
		ql.writer.Flush()
	}
	if ql.logFile != nil {
		ql.logFile.Close()
	}

	ql.enabled = false
	ql.logger.Info("DNS query logger stopped")
}

// LogQuery logs a DNS query
func (ql *QueryLogger) LogQuery(event *types.DNSQueryEvent) {
	if !ql.enabled || !ql.config.LogQueries {
		return
	}

	// Check if we should log this query
	if !ql.shouldLogQuery(event) {
		return
	}

	// Add to buffer for batch writing
	ql.bufferMutex.Lock()
	ql.eventBuffer = append(ql.eventBuffer, event)
	shouldFlush := len(ql.eventBuffer) >= ql.config.BufferSize
	ql.bufferMutex.Unlock()

	if shouldFlush {
		ql.flushBuffer()
	}
}

// LogResponse logs a DNS response
func (ql *QueryLogger) LogResponse(event *types.DNSResponseEvent, blocked bool, reason string) {
	if !ql.enabled || !ql.config.LogResponses {
		return
	}

	// Convert response to log entry
	entry := ql.createResponseLogEntry(event)
	entry.Blocked = blocked
	entry.Reason = reason

	// Write immediately (responses are less frequent)
	ql.writeLogEntry(entry)
	atomic.AddUint64(&ql.stats.ResponsesLogged, 1)
}

// shouldLogQuery determines if a query should be logged
func (ql *QueryLogger) shouldLogQuery(event *types.DNSQueryEvent) bool {
	// Check private IP filter
	if !ql.config.LogPrivateIPs && ql.isPrivateIP(event.SourceIP) {
		return false
	}

	// Check blocked-only filter
	if ql.config.LogBlockedOnly {
		// Requires integration with DNSFilter block decisions (not yet wired up).
		return false
	}

	return true
}

// isPrivateIP checks if an IP is private
func (ql *QueryLogger) isPrivateIP(ip net.IP) bool {
	// Check for private IP ranges
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// 127.0.0.0/8 (localhost)
		if ip4[0] == 127 {
			return true
		}
	}

	return false
}

// createQueryLogEntry creates a log entry from a query event
func (ql *QueryLogger) createQueryLogEntry(event *types.DNSQueryEvent) *LogEntry {
	entry := &LogEntry{
		Type:       "query",
		SourceIP:   event.SourceIP.String(),
		SourcePort: event.SourcePort,
		DestIP:     event.DestIP.String(),
		DestPort:   event.DestPort,
		QueryID:    event.QueryID,
		Domain:     event.Domain,
		PacketSize: event.PacketSize,
	}

	if ql.config.IncludeTimestamp {
		entry.Timestamp = event.Timestamp
	}
	if ql.config.IncludePID {
		entry.PID = event.PID
		entry.TID = event.TID
	}
	entry.QueryType = event.QueryType
	entry.QueryClass = event.QueryClass

	return entry
}

// createResponseLogEntry creates a log entry from a response event
func (ql *QueryLogger) createResponseLogEntry(event *types.DNSResponseEvent) *LogEntry {
	entry := &LogEntry{
		Type:         "response",
		QueryID:      event.QueryID,
		Domain:       event.Domain,
		ResponseCode: event.ResponseCode,
		AnswerCount:  event.AnswerCount,
		PacketSize:   event.PacketSize,
	}

	if ql.config.IncludeTimestamp {
		entry.Timestamp = event.Timestamp
	}
	if ql.config.IncludeResponseTime {
		entry.ResponseTime = event.ResponseTime.String()
	}

	return entry
}

// writeLogEntry writes a log entry
func (ql *QueryLogger) writeLogEntry(entry *LogEntry) error {
	if ql.writer == nil {
		return fmt.Errorf("log writer not initialized")
	}

	var line []byte
	var err error

	switch ql.config.LogFormat {
	case "json":
		line, err = json.Marshal(entry)
		if err != nil {
			atomic.AddUint64(&ql.stats.Errors, 1)
			return err
		}
		line = append(line, '\n')
	case "text":
		line = []byte(ql.formatTextEntry(entry))
	default:
		return fmt.Errorf("unsupported log format: %s", ql.config.LogFormat)
	}

	// Write to file
	n, err := ql.writer.Write(line)
	if err != nil {
		atomic.AddUint64(&ql.stats.Errors, 1)
		return err
	}

	atomic.AddUint64(&ql.stats.BytesWritten, uint64(n))

	// Check if we need to rotate
	if ql.config.MaxFileSize > 0 {
		if stat, err := ql.logFile.Stat(); err == nil {
			if stat.Size() >= ql.config.MaxFileSize {
				ql.rotateLogFile()
			}
		}
	}

	return nil
}

// formatTextEntry formats a log entry as text
func (ql *QueryLogger) formatTextEntry(entry *LogEntry) string {
	var parts []string

	if ql.config.IncludeTimestamp {
		parts = append(parts, entry.Timestamp.Format(time.RFC3339))
	}

	parts = append(parts,
		fmt.Sprintf("type=%s", entry.Type),
		fmt.Sprintf("src=%s:%d", entry.SourceIP, entry.SourcePort),
		fmt.Sprintf("dst=%s:%d", entry.DestIP, entry.DestPort),
		fmt.Sprintf("query_id=%d", entry.QueryID),
		fmt.Sprintf("domain=%s", entry.Domain),
	)

	if entry.Type == "query" {
		parts = append(parts,
			fmt.Sprintf("type=%d", entry.QueryType),
			fmt.Sprintf("class=%d", entry.QueryClass),
		)
	} else {
		parts = append(parts,
			fmt.Sprintf("rcode=%d", entry.ResponseCode),
			fmt.Sprintf("answers=%d", entry.AnswerCount),
		)
		if entry.ResponseTime != "" {
			parts = append(parts, fmt.Sprintf("rtt=%s", entry.ResponseTime))
		}
	}

	if ql.config.IncludePID && entry.PID > 0 {
		parts = append(parts,
			fmt.Sprintf("pid=%d", entry.PID),
			fmt.Sprintf("tid=%d", entry.TID),
		)
	}

	parts = append(parts, fmt.Sprintf("size=%d", entry.PacketSize))

	return fmt.Sprintf("%s\n", strings.Join(parts, " "))
}

// flushWorker periodically flushes the buffer
func (ql *QueryLogger) flushWorker() {
	ticker := time.NewTicker(ql.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ql.ctx.Done():
			return
		case <-ticker.C:
			ql.flushBuffer()
		}
	}
}

// flushBuffer flushes the event buffer
func (ql *QueryLogger) flushBuffer() {
	ql.bufferMutex.Lock()
	if len(ql.eventBuffer) == 0 {
		ql.bufferMutex.Unlock()
		return
	}

	// Copy buffer and clear
	events := make([]*types.DNSQueryEvent, len(ql.eventBuffer))
	copy(events, ql.eventBuffer)
	ql.eventBuffer = ql.eventBuffer[:0]
	ql.bufferMutex.Unlock()

	// Write events
	for _, event := range events {
		entry := ql.createQueryLogEntry(event)
		if err := ql.writeLogEntry(entry); err != nil {
			ql.logger.Error("Failed to write query log", "error", err)
			atomic.AddUint64(&ql.stats.Errors, 1)
		} else {
			atomic.AddUint64(&ql.stats.QueriesLogged, 1)
		}
	}

	// Flush writer
	if ql.writer != nil {
		ql.writer.Flush()
	}

	atomic.AddUint64(&ql.stats.BufferFlushes, 1)
	ql.stats.LastFlush = time.Now()
}

// openLogFile opens the log file
func (ql *QueryLogger) openLogFile() error {
	// Check if file exists and get its size
	if stat, err := os.Stat(ql.config.LogFilePath); err == nil {
		if stat.Size() >= ql.config.MaxFileSize {
			ql.rotateLogFile()
		}
	}

	// Open file in append mode
	file, err := os.OpenFile(ql.config.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	ql.logFile = file
	ql.writer = bufio.NewWriterSize(file, 64*1024) // 64KB buffer

	return nil
}

// rotateLogFile rotates the log file
func (ql *QueryLogger) rotateLogFile() {
	if ql.logFile == nil {
		return
	}

	// Flush and close current file
	ql.writer.Flush()
	ql.logFile.Close()

	// Move current file to backup
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", ql.config.LogFilePath, timestamp)
	if err := os.Rename(ql.config.LogFilePath, backupPath); err != nil {
		ql.logger.Error("Failed to rotate log file", "error", err)
		atomic.AddUint64(&ql.stats.Errors, 1)
		return
	}

	// Compress backup if enabled
	if ql.config.CompressBackups {
		go ql.compressLogFile(backupPath)
	}

	// Clean up old backups
	go ql.cleanupOldBackups()

	// Open new log file
	if err := ql.openLogFile(); err != nil {
		ql.logger.Error("Failed to open new log file after rotation", "error", err)
		atomic.AddUint64(&ql.stats.Errors, 1)
	}

	atomic.AddUint64(&ql.stats.FilesRotated, 1)
	ql.stats.LastRotation = time.Now()

	ql.logger.Info("Log file rotated", "backup", backupPath)
}

// compressLogFile compresses a log file using gzip
func (ql *QueryLogger) compressLogFile(filePath string) {
	ql.logger.Debug("Compressing log file", "file", filePath)

	// Open source file
	src, err := os.Open(filePath)
	if err != nil {
		ql.logger.Error("Failed to open log file for compression", "file", filePath, "error", err)
		return
	}
	defer src.Close()

	// Create destination file
	dstPath := filePath + ".gz"
	dst, err := os.Create(dstPath)
	if err != nil {
		ql.logger.Error("Failed to create compressed log file", "file", dstPath, "error", err)
		return
	}
	defer dst.Close()

	// Create gzip writer
	gw := gzip.NewWriter(dst)
	defer gw.Close()

	// Copy data
	if _, err := io.Copy(gw, src); err != nil {
		ql.logger.Error("Failed to compress log file", "file", filePath, "error", err)
		return
	}

	// Close files before removing source
	gw.Close()
	src.Close()

	// Remove source file
	if err := os.Remove(filePath); err != nil {
		ql.logger.Error("Failed to remove source log file after compression", "file", filePath, "error", err)
	}
}

// cleanupOldBackups removes old backup files keeping only MaxBackups
func (ql *QueryLogger) cleanupOldBackups() {
	if ql.config.MaxBackups <= 0 {
		return
	}

	ql.logger.Debug("Cleaning up old backup files")

	// Find all backup files (both raw and compressed)
	pattern := ql.config.LogFilePath + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		ql.logger.Error("Failed to glob backup files", "pattern", pattern, "error", err)
		return
	}

	if len(matches) <= ql.config.MaxBackups {
		return
	}

	// Sort by modification time (oldest first)
	sort.Slice(matches, func(i, j int) bool {
		si, _ := os.Stat(matches[i])
		sj, _ := os.Stat(matches[j])
		if si == nil || sj == nil {
			return matches[i] < matches[j]
		}
		return si.ModTime().Before(sj.ModTime())
	})

	// Remove oldest files
	numToRemove := len(matches) - ql.config.MaxBackups
	for i := 0; i < numToRemove; i++ {
		ql.logger.Debug("Removing old backup file", "file", matches[i])
		if err := os.Remove(matches[i]); err != nil {
			ql.logger.Error("Failed to remove old backup file", "file", matches[i], "error", err)
		}
	}
}

// GetStatistics returns query logger statistics
func (ql *QueryLogger) GetStatistics() interface{} {
	ql.mutex.RLock()
	defer ql.mutex.RUnlock()

	stats := *ql.stats
	return &stats
}

// IsEnabled returns whether the logger is enabled
func (ql *QueryLogger) IsEnabled() bool {
	ql.mutex.RLock()
	defer ql.mutex.RUnlock()
	return ql.enabled
}
