// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package socket

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"grimm.is/flywall/internal/ebpf/types"
	"grimm.is/flywall/internal/logging"

	"gopkg.in/yaml.v3"
)

// DeviceDatabase manages persistent storage of device information
type DeviceDatabase struct {
	// Configuration
	config *DeviceDatabaseConfig

	// State
	mutex   sync.RWMutex
	enabled bool

	// Database storage
	devices   map[string]*types.DeviceInfo
	devMutex  sync.RWMutex
	createdAt time.Time

	// Statistics
	stats *DeviceDatabaseStats

	// Logger
	logger *logging.Logger

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// DeviceDatabaseConfig holds configuration for device database
type DeviceDatabaseConfig struct {
	// Database settings
	Enabled      bool   `hcl:"enabled,optional"`
	DatabasePath string `hcl:"database_path,optional"`
	BackupPath   string `hcl:"backup_path,optional"`

	// Persistence settings
	AutoSave       bool          `hcl:"auto_save,optional"`
	SaveInterval   time.Duration `hcl:"save_interval,optional"`
	BackupInterval time.Duration `hcl:"backup_interval,optional"`
	MaxBackups     int           `hcl:"max_backups,optional"`

	// Data retention
	RetentionPeriod   time.Duration `hcl:"retention_period,optional"`
	ArchiveOldDevices bool          `hcl:"archive_old_devices,optional"`

	// Export settings
	ExportFormat string `hcl:"export_format,optional"` // json, csv, yaml
	ExportOnSave bool   `hcl:"export_on_save,optional"`
	ExportPath   string `hcl:"export_path,optional"`

	// Integration
	SyncWithDiscovery bool          `hcl:"sync_with_discovery,optional"`
	SyncInterval      time.Duration `hcl:"sync_interval,optional"`
}

// DeviceDatabaseStats holds statistics for the device database
type DeviceDatabaseStats struct {
	DevicesStored   uint64    `json:"devices_stored"`
	DevicesUpdated  uint64    `json:"devices_updated"`
	DevicesArchived uint64    `json:"devices_archived"`
	DevicesDeleted  uint64    `json:"devices_deleted"`
	SavesPerformed  uint64    `json:"saves_performed"`
	BackupsCreated  uint64    `json:"backups_created"`
	ExportsCreated  uint64    `json:"exports_created"`
	DatabaseSize    uint64    `json:"database_size"`
	LastSave        time.Time `json:"last_save"`
	LastBackup      time.Time `json:"last_backup"`
	LastExport      time.Time `json:"last_export"`
}

// DatabaseVersion tracks database schema version
const DatabaseVersion = 1

// DatabaseMetadata holds database metadata
type DatabaseMetadata struct {
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeviceCount int       `json:"device_count"`
}

// DefaultDeviceDatabaseConfig returns default configuration
func DefaultDeviceDatabaseConfig() *DeviceDatabaseConfig {
	return &DeviceDatabaseConfig{
		Enabled:           true,
		DatabasePath:      "/var/lib/flywall/devices.db",
		BackupPath:        "/var/lib/flywall/backups",
		AutoSave:          true,
		SaveInterval:      5 * time.Minute,
		BackupInterval:    24 * time.Hour,
		MaxBackups:        7,
		RetentionPeriod:   90 * 24 * time.Hour, // 90 days
		ArchiveOldDevices: true,
		ExportFormat:      "json",
		ExportOnSave:      false,
		ExportPath:        "/var/lib/flywall/exports",
		SyncWithDiscovery: true,
		SyncInterval:      1 * time.Minute,
	}
}

// NewDeviceDatabase creates a new device database
func NewDeviceDatabase(logger *logging.Logger, config *DeviceDatabaseConfig) *DeviceDatabase {
	if config == nil {
		config = DefaultDeviceDatabaseConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	ddb := &DeviceDatabase{
		config: config,
		stats: &DeviceDatabaseStats{
			LastSave: time.Now(),
		},
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		devices:   make(map[string]*types.DeviceInfo),
		createdAt: time.Now(),
	}

	return ddb
}

// Start starts the device database
func (ddb *DeviceDatabase) Start() error {
	ddb.mutex.Lock()
	defer ddb.mutex.Unlock()

	if !ddb.config.Enabled {
		ddb.logger.Info("Device database disabled")
		return nil
	}

	ddb.logger.Info("Starting device database")

	// Create directories
	if err := ddb.createDirectories(); err != nil {
		return err
	}

	// Load existing database
	if err := ddb.loadDatabase(); err != nil {
		ddb.logger.Warn("Failed to load existing database", "error", err)
		// Continue with empty database
	}

	// Start auto-save goroutine
	if ddb.config.AutoSave {
		go ddb.autoSaveWorker()
	}

	// Start backup goroutine
	if ddb.config.BackupInterval > 0 {
		go ddb.backupWorker()
	}

	// Start sync goroutine
	if ddb.config.SyncWithDiscovery {
		go ddb.syncWorker()
	}

	ddb.enabled = true
	ddb.logger.Info("Device database started",
		"devices_loaded", len(ddb.devices))

	return nil
}

// Stop stops the device database
func (ddb *DeviceDatabase) Stop() {
	ddb.mutex.Lock()
	defer ddb.mutex.Unlock()

	if !ddb.enabled {
		return
	}

	ddb.logger.Info("Stopping device database")

	// Final save
	if ddb.config.AutoSave {
		if err := ddb.saveDatabase(); err != nil {
			ddb.logger.Error("Failed to save database on shutdown", "error", err)
		}
	}

	// Cancel context
	ddb.cancel()

	ddb.enabled = false
	ddb.logger.Info("Device database stopped")
}

// StoreDevice stores or updates a device in the database
func (ddb *DeviceDatabase) StoreDevice(device *types.DeviceInfo) error {
	if !ddb.enabled {
		return nil
	}

	ddb.devMutex.Lock()
	defer ddb.devMutex.Unlock()

	key := device.MACAddress
	if key == "" {
		key = device.HostName
	}
	if key == "" {
		return fmt.Errorf("device has no identifier")
	}

	existing, exists := ddb.devices[key]
	if exists {
		// Update existing device
		atomic.AddUint64(&ddb.stats.DevicesUpdated, 1)

		// Preserve first seen time
		device.FirstSeen = existing.FirstSeen

		// Update last seen
		device.LastSeen = time.Now()
	} else {
		// New device
		atomic.AddUint64(&ddb.stats.DevicesStored, 1)
		device.FirstSeen = time.Now()
		device.LastSeen = time.Now()
	}

	// Store device
	ddb.devices[key] = device

	// Auto-save if configured
	if ddb.config.AutoSave && ddb.config.SaveInterval == 0 {
		go ddb.saveDatabase()
	}

	return nil
}

// GetDevice retrieves a device from the database
func (ddb *DeviceDatabase) GetDevice(identifier string) (*types.DeviceInfo, error) {
	if !ddb.enabled {
		return nil, fmt.Errorf("database disabled")
	}

	ddb.devMutex.RLock()
	defer ddb.devMutex.RUnlock()

	device, exists := ddb.devices[identifier]
	if !exists {
		return nil, fmt.Errorf("device not found")
	}

	// Return a copy
	deviceCopy := *device
	return &deviceCopy, nil
}

// ListDevices returns all devices in the database
func (ddb *DeviceDatabase) ListDevices() ([]types.DeviceInfo, error) {
	if !ddb.enabled {
		return nil, fmt.Errorf("database disabled")
	}

	ddb.devMutex.RLock()
	defer ddb.devMutex.RUnlock()

	devices := make([]types.DeviceInfo, 0, len(ddb.devices))
	for _, device := range ddb.devices {
		devices = append(devices, *device)
	}

	return devices, nil
}

// DeleteDevice removes a device from the database
func (ddb *DeviceDatabase) DeleteDevice(identifier string) error {
	if !ddb.enabled {
		return nil
	}

	ddb.devMutex.Lock()
	defer ddb.devMutex.Unlock()

	if _, exists := ddb.devices[identifier]; !exists {
		return fmt.Errorf("device not found")
	}

	delete(ddb.devices, identifier)
	atomic.AddUint64(&ddb.stats.DevicesDeleted, 1)

	return nil
}

// createDirectories creates necessary directories
func (ddb *DeviceDatabase) createDirectories() error {
	// Create database directory
	if ddb.config.DatabasePath != "" {
		dir := filepath.Dir(ddb.config.DatabasePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Create backup directory
	if ddb.config.BackupPath != "" {
		if err := os.MkdirAll(ddb.config.BackupPath, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
	}

	// Create export directory
	if ddb.config.ExportPath != "" {
		if err := os.MkdirAll(ddb.config.ExportPath, 0755); err != nil {
			return fmt.Errorf("failed to create export directory: %w", err)
		}
	}

	return nil
}

// loadDatabase loads the database from disk
func (ddb *DeviceDatabase) loadDatabase() error {
	if ddb.config.DatabasePath == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(ddb.config.DatabasePath); os.IsNotExist(err) {
		ddb.logger.Info("Database file does not exist, starting with empty database")
		return nil
	}

	// Open file
	file, err := os.Open(ddb.config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database file: %w", err)
	}
	defer file.Close()

	// Read metadata
	var metadata DatabaseMetadata
	if err := json.NewDecoder(file).Decode(&metadata); err != nil {
		return fmt.Errorf("failed to read database metadata: %w", err)
	}

	// Check version compatibility
	if metadata.Version > DatabaseVersion {
		return fmt.Errorf("database version %d is newer than supported version %d",
			metadata.Version, DatabaseVersion)
	}

	// Read devices
	var devices map[string]*types.DeviceInfo
	if err := json.NewDecoder(file).Decode(&devices); err != nil {
		return fmt.Errorf("failed to read devices: %w", err)
	}

	ddb.devices = devices
	ddb.createdAt = metadata.CreatedAt
	ddb.logger.Info("Database loaded successfully",
		"devices", metadata.DeviceCount,
		"version", metadata.Version,
		"updated_at", metadata.UpdatedAt)

	return nil
}

// saveDatabase saves the database to disk
func (ddb *DeviceDatabase) saveDatabase() error {
	if ddb.config.DatabasePath == "" {
		return nil
	}

	ddb.devMutex.RLock()
	defer ddb.devMutex.RUnlock()

	// Create temporary file
	tempFile := ddb.config.DatabasePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Write metadata
	metadata := DatabaseMetadata{
		Version:     DatabaseVersion,
		CreatedAt:   ddb.createdAt,
		UpdatedAt:   time.Now(),
		DeviceCount: len(ddb.devices),
	}

	if err := json.NewEncoder(file).Encode(metadata); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write devices
	if err := json.NewEncoder(file).Encode(ddb.devices); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to write devices: %w", err)
	}

	file.Close()

	// Atomic rename
	if err := os.Rename(tempFile, ddb.config.DatabasePath); err != nil {
		return fmt.Errorf("failed to rename database file: %w", err)
	}

	// Update statistics
	atomic.AddUint64(&ddb.stats.SavesPerformed, 1)
	ddb.stats.LastSave = time.Now()

	// Export if configured
	if ddb.config.ExportOnSave {
		go ddb.exportDatabase()
	}

	ddb.logger.Debug("Database saved successfully",
		"devices", metadata.DeviceCount)

	return nil
}

// backupDatabase creates a backup of the database
func (ddb *DeviceDatabase) backupDatabase() error {
	if ddb.config.DatabasePath == "" || ddb.config.BackupPath == "" {
		return nil
	}

	// Create backup filename
	timestamp := time.Now().Format("20060102-150405")
	backupFile := filepath.Join(ddb.config.BackupPath,
		fmt.Sprintf("devices-%s.db", timestamp))

	// Copy database file
	if err := copyFile(ddb.config.DatabasePath, backupFile); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Update statistics
	atomic.AddUint64(&ddb.stats.BackupsCreated, 1)
	ddb.stats.LastBackup = time.Now()

	// Clean up old backups
	go ddb.cleanupOldBackups()

	ddb.logger.Info("Database backup created", "file", backupFile)
	return nil
}

// exportDatabase exports the database in the configured format
func (ddb *DeviceDatabase) exportDatabase() error {
	if ddb.config.ExportPath == "" {
		return nil
	}

	timestamp := time.Now().Format("20060102-150405")
	var filename string

	switch ddb.config.ExportFormat {
	case "json":
		filename = filepath.Join(ddb.config.ExportPath,
			fmt.Sprintf("devices-%s.json", timestamp))
		return ddb.exportJSON(filename)
	case "csv":
		filename = filepath.Join(ddb.config.ExportPath,
			fmt.Sprintf("devices-%s.csv", timestamp))
		return ddb.exportCSV(filename)
	case "yaml":
		filename = filepath.Join(ddb.config.ExportPath,
			fmt.Sprintf("devices-%s.yaml", timestamp))
		return ddb.exportYAML(filename)
	default:
		return fmt.Errorf("unsupported export format: %s", ddb.config.ExportFormat)
	}
}

// exportJSON exports database as JSON
func (ddb *DeviceDatabase) exportJSON(filename string) error {
	ddb.devMutex.RLock()
	defer ddb.devMutex.RUnlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(ddb.devices); err != nil {
		return err
	}

	atomic.AddUint64(&ddb.stats.ExportsCreated, 1)
	ddb.stats.LastExport = time.Now()

	return nil
}

// exportCSV exports database as CSV
func (ddb *DeviceDatabase) exportCSV(filename string) error {
	ddb.devMutex.RLock()
	defer ddb.devMutex.RUnlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"MACAddress", "IPAddress", "HostName", "Vendor", "DeviceType", "FirstSeen", "LastSeen", "LeaseExpiry"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Get sorted keys for consistent output
	keys := make([]string, 0, len(ddb.devices))
	for k := range ddb.devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		dev := ddb.devices[k]
		expiry := ""
		if dev.LeaseExpiry != nil {
			expiry = dev.LeaseExpiry.Format(time.RFC3339)
		}

		row := []string{
			dev.MACAddress,
			dev.IPAddress.String(),
			dev.HostName,
			dev.Vendor,
			dev.DeviceType,
			dev.FirstSeen.Format(time.RFC3339),
			dev.LastSeen.Format(time.RFC3339),
			expiry,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	atomic.AddUint64(&ddb.stats.ExportsCreated, 1)
	ddb.stats.LastExport = time.Now()

	return nil
}

// exportYAML exports database as YAML
func (ddb *DeviceDatabase) exportYAML(filename string) error {
	ddb.devMutex.RLock()
	defer ddb.devMutex.RUnlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	if err := encoder.Encode(ddb.devices); err != nil {
		return err
	}

	atomic.AddUint64(&ddb.stats.ExportsCreated, 1)
	ddb.stats.LastExport = time.Now()

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = destination.ReadFrom(source)
	return err
}

// cleanupOldBackups removes old backup files
func (ddb *DeviceDatabase) cleanupOldBackups() {
	if ddb.config.MaxBackups <= 0 {
		return
	}

	files, err := os.ReadDir(ddb.config.BackupPath)
	if err != nil {
		return
	}

	// Sort files by name (timestamp)
	type fileInfo struct {
		name    string
		modTime time.Time
	}

	var backupFiles []fileInfo
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".db" {
			info, err := file.Info()
			if err != nil {
				continue
			}
			backupFiles = append(backupFiles, fileInfo{
				name:    file.Name(),
				modTime: info.ModTime(),
			})
		}
	}

	// Remove oldest if too many
	if len(backupFiles) > ddb.config.MaxBackups {
		// Sort by modification time (oldest first)
		for i := 0; i < len(backupFiles)-1; i++ {
			for j := i + 1; j < len(backupFiles); j++ {
				if backupFiles[i].modTime.After(backupFiles[j].modTime) {
					backupFiles[i], backupFiles[j] = backupFiles[j], backupFiles[i]
				}
			}
		}

		// Remove excess files
		for i := 0; i < len(backupFiles)-ddb.config.MaxBackups; i++ {
			filePath := filepath.Join(ddb.config.BackupPath, backupFiles[i].name)
			os.Remove(filePath)
		}
	}
}

// autoSaveWorker periodically saves the database
func (ddb *DeviceDatabase) autoSaveWorker() {
	if ddb.config.SaveInterval <= 0 {
		return
	}

	ticker := time.NewTicker(ddb.config.SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ddb.ctx.Done():
			return
		case <-ticker.C:
			if err := ddb.saveDatabase(); err != nil {
				ddb.logger.Error("Auto-save failed", "error", err)
			}
		}
	}
}

// backupWorker periodically creates backups
func (ddb *DeviceDatabase) backupWorker() {
	ticker := time.NewTicker(ddb.config.BackupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ddb.ctx.Done():
			return
		case <-ticker.C:
			if err := ddb.backupDatabase(); err != nil {
				ddb.logger.Error("Backup failed", "error", err)
			}
		}
	}
}

// syncWorker synchronizes with device discovery.
// Stub: waits for device discovery module to be wired in.
func (ddb *DeviceDatabase) syncWorker() {
	<-ddb.ctx.Done()
}

// GetStatistics returns database statistics
func (ddb *DeviceDatabase) GetStatistics() *DeviceDatabaseStats {
	ddb.mutex.RLock()
	defer ddb.mutex.RUnlock()

	stats := *ddb.stats
	stats.DatabaseSize = uint64(len(ddb.devices))
	return &stats
}

// IsEnabled returns whether the database is enabled
func (ddb *DeviceDatabase) IsEnabled() bool {
	ddb.mutex.RLock()
	defer ddb.mutex.RUnlock()
	return ddb.enabled
}
