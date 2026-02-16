// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package install

import (
	"os"
	"path/filepath"

	"grimm.is/flywall/internal/brand"
)

// Exported variables for backward compatibility and convenience
var (
	DefaultConfigDir string
	DefaultStateDir  string
	DefaultLogDir    string
	DefaultCacheDir  string
	DefaultRunDir    string
	DefaultShareDir  string

	// Build-time path overrides (set via -ldflags)
	// These allow distributions to override /opt/flywall defaults back to /etc, /var, etc.
	BuildDefaultConfigDir = ""
	BuildDefaultStateDir  = ""
	BuildDefaultLogDir    = ""
	BuildDefaultCacheDir  = ""
	BuildDefaultRunDir    = ""
	BuildDefaultShareDir  = ""
)

func init() {
	b := brand.Get()

	// Apply build-time overrides if set, otherwise use JSON defaults
	if BuildDefaultConfigDir != "" {
		DefaultConfigDir = BuildDefaultConfigDir
	} else {
		DefaultConfigDir = b.DefaultConfigDir
	}

	if BuildDefaultStateDir != "" {
		DefaultStateDir = BuildDefaultStateDir
	} else {
		DefaultStateDir = b.DefaultStateDir
	}

	if BuildDefaultLogDir != "" {
		DefaultLogDir = BuildDefaultLogDir
	} else {
		DefaultLogDir = b.DefaultLogDir
	}

	if BuildDefaultCacheDir != "" {
		DefaultCacheDir = BuildDefaultCacheDir
	} else {
		DefaultCacheDir = b.DefaultCacheDir
	}

	if BuildDefaultRunDir != "" {
		DefaultRunDir = BuildDefaultRunDir
	} else {
		DefaultRunDir = b.DefaultRunDir
	}

	if BuildDefaultShareDir != "" {
		DefaultShareDir = BuildDefaultShareDir
	} else {
		DefaultShareDir = b.DefaultShareDir
	}
}

// GetStateDir returns the state directory, checking env vars first.
// Priority: FLYWALL_STATE_DIR > FLYWALL_PREFIX/state > DefaultStateDir
func GetStateDir() string {
	if dir := os.Getenv(brand.ConfigEnvPrefix + "_STATE_DIR"); dir != "" {
		return dir
	}
	if prefix := os.Getenv(brand.ConfigEnvPrefix + "_PREFIX"); prefix != "" {
		return filepath.Join(prefix, "state")
	}
	return DefaultStateDir
}

// GetLogDir returns the log directory, checking env vars first.
// Priority: FLYWALL_LOG_DIR > FLYWALL_PREFIX/log > DefaultLogDir
func GetLogDir() string {
	if dir := os.Getenv(brand.ConfigEnvPrefix + "_LOG_DIR"); dir != "" {
		return dir
	}
	if prefix := os.Getenv(brand.ConfigEnvPrefix + "_PREFIX"); prefix != "" {
		return filepath.Join(prefix, "log")
	}
	return DefaultLogDir
}

// GetConfigDir returns the config directory, checking env vars first.
// Priority: FLYWALL_CONFIG_DIR > FLYWALL_PREFIX/config > DefaultConfigDir
func GetConfigDir() string {
	if dir := os.Getenv(brand.ConfigEnvPrefix + "_CONFIG_DIR"); dir != "" {
		return dir
	}
	if prefix := os.Getenv(brand.ConfigEnvPrefix + "_PREFIX"); prefix != "" {
		return filepath.Join(prefix, "config")
	}
	return DefaultConfigDir
}

// GetCacheDir returns the cache directory, checking env vars first.
// Priority: FLYWALL_CACHE_DIR > FLYWALL_PREFIX/cache > DefaultCacheDir
func GetCacheDir() string {
	if dir := os.Getenv(brand.ConfigEnvPrefix + "_CACHE_DIR"); dir != "" {
		return dir
	}
	if prefix := os.Getenv(brand.ConfigEnvPrefix + "_PREFIX"); prefix != "" {
		return filepath.Join(prefix, "cache")
	}
	return DefaultCacheDir
}

// GetRunDir returns the runtime directory for sockets and PID files.
// Priority: FLYWALL_RUN_DIR > FLYWALL_PREFIX/run > DefaultRunDir
func GetRunDir() string {
	if dir := os.Getenv(brand.ConfigEnvPrefix + "_RUN_DIR"); dir != "" {
		return dir
	}
	if prefix := os.Getenv(brand.ConfigEnvPrefix + "_PREFIX"); prefix != "" {
		return filepath.Join(prefix, "run")
	}
	return DefaultRunDir
}

// GetShareDir returns the shared directory for persistent data (geoip, ipsets).
// Priority: FLYWALL_SHARE_DIR > FLYWALL_PREFIX/share > DefaultShareDir
func GetShareDir() string {
	if dir := os.Getenv(brand.ConfigEnvPrefix + "_SHARE_DIR"); dir != "" {
		return dir
	}
	if prefix := os.Getenv(brand.ConfigEnvPrefix + "_PREFIX"); prefix != "" {
		return filepath.Join(prefix, "share")
	}
	return DefaultShareDir
}

// GetSocketPath returns the full path to the control plane socket.
// The socket name includes the brand name for uniqueness.
// Returns: /var/run/flywall-ctl.sock (or equivalent based on env/prefix)
func GetSocketPath() string {
	if path := os.Getenv(brand.ConfigEnvPrefix + "_CTL_SOCKET"); path != "" {
		return path
	}
	runDir := GetRunDir()
	// Use format: <lowerName>-<socketName> e.g., flywall-ctl.sock
	return filepath.Join(runDir, brand.LowerName+"-"+brand.SocketName)
}
