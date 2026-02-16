// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"grimm.is/flywall/internal/logging"
	"github.com/stretchr/testify/assert"
)

func TestQueryLogger_RotationAndCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "query_logger_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "dns_queries.log")
	
	config := &QueryLoggerConfig{
		LogFilePath:     logPath,
		MaxFileSize:     100, // Very small for easy rotation
		MaxBackups:      2,
		CompressBackups: true,
		FlushInterval:   100 * time.Millisecond,
		BufferSize:      1,
		LogFormat:       "json",
	}

	logger := logging.New(logging.DefaultConfig())
	ql := NewQueryLogger(logger, config)

	err = ql.Start()
	assert.NoError(t, err)
	defer ql.Stop()

	// Write enough data to trigger multiple rotations
	for i := 0; i < 10; i++ {
		// Mock write entry
		err = ql.writeLogEntry(&LogEntry{
			Type:   "query",
			Domain: "example.com",
		})
		assert.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	// Give some time for async compression and cleanup
	time.Sleep(500 * time.Millisecond)

	// Check backups
	files, err := filepath.Glob(logPath + ".*")
	assert.NoError(t, err)
	
	// Should have exactly MaxBackups files
	assert.LessOrEqual(t, len(files), config.MaxBackups)

	// Verify they are compressed (ending in .gz)
	for _, f := range files {
		assert.True(t, filepath.Ext(f) == ".gz", "File %s should be compressed", f)
		
		// Verify gzip format
		file, err := os.Open(f)
		assert.NoError(t, err)
		zr, err := gzip.NewReader(file)
		assert.NoError(t, err)
		_, err = io.ReadAll(zr)
		assert.NoError(t, err)
		zr.Close()
		file.Close()
	}
}
