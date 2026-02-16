// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// Package brand provides centralized branding constants for the firewall.
// This makes it easy to fork or white-label the product by changing brand.json.
//
// The brand identity is loaded from brand.json at compile time via go:embed.
// This allows other tools (scripts, docs generators) to read the same file.
package brand

import (
	_ "embed"
	"encoding/json"
)

//go:embed brand.json
var brandJSON []byte

// Brand holds all branding information
type Brand struct {
	Name             string `json:"name"`
	LowerName        string `json:"lowerName"`
	Vendor           string `json:"vendor"`
	Website          string `json:"website"`
	Repository       string `json:"repository"`
	Description      string `json:"description"`
	Tagline          string `json:"tagline"`
	ConfigEnvPrefix  string `json:"configEnvPrefix"`
	DefaultConfigDir string `json:"defaultConfigDir"`
	DefaultStateDir  string `json:"defaultStateDir"`
	DefaultLogDir    string `json:"defaultLogDir"`
	DefaultCacheDir  string `json:"defaultCacheDir"`
	DefaultRunDir    string `json:"defaultRunDir"`
	DefaultShareDir  string `json:"defaultShareDir"`
	APIKeyPrefix     string `json:"apiKeyPrefix"`
	SocketName       string `json:"socketName"`
	BinaryName       string `json:"binaryName"`
	ServiceName      string `json:"serviceName"`
	ConfigFileName   string `json:"configFileName"`
	Copyright        string `json:"copyright"`
	License          string `json:"license"`
	DefaultCloudURL  string `json:"defaultCloudURL"`
}

var b Brand

func init() {
	if err := json.Unmarshal(brandJSON, &b); err != nil {
		panic("failed to parse brand.json: " + err.Error())
	}

	// Initialize exported variables after JSON is parsed
	Name = b.Name
	LowerName = b.LowerName
	Vendor = b.Vendor
	Website = b.Website
	Repository = b.Repository
	Description = b.Description
	Tagline = b.Tagline
	ConfigEnvPrefix = b.ConfigEnvPrefix

	// Path initialization moved to internal/install package

	APIKeyPrefix = b.APIKeyPrefix
	SocketName = b.SocketName
	BinaryName = b.BinaryName
	ServiceName = b.ServiceName
	ConfigFileName = b.ConfigFileName
	Copyright = b.Copyright
	License = b.License
	DefaultCloudURL = b.DefaultCloudURL
}

// Exported variables for backward compatibility and convenience
var (
	Name            string
	LowerName       string
	Vendor          string
	Website         string
	Repository      string
	Description     string
	Tagline         string
	ConfigEnvPrefix string
	// Paths moved to internal/install
	APIKeyPrefix    string
	SocketName      string
	BinaryName      string
	ServiceName     string
	ConfigFileName  string
	Copyright       string
	License         string
	DefaultCloudURL string

	// Version is set at build time via -ldflags
	Version      = "dev"
	BuildTime    = "unknown"
	BuildArch    = "unknown"
	GitCommit    = "unknown"
	GitBranch    = "unknown"
	GitMergeBase = "unknown"
)

// Get returns the full Brand struct
func Get() Brand {
	return b
}

// UserAgent returns a User-Agent string for HTTP requests
func UserAgent(version string) string {
	if version == "" {
		version = "dev"
	}
	return Name + "/" + version
}

// APIKeyPrefixFull returns the API key prefix with trailing underscore.
// This is the format used in actual API keys (e.g., "gfw_").
// Falls back to LowerName if apiKeyPrefix is not configured.
func APIKeyPrefixFull() string {
	prefix := APIKeyPrefix
	if prefix == "" {
		prefix = LowerName
	}
	return prefix + "_"
}
