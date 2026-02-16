// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// WebConfig configures the public-facing web server (Proxy).
type WebConfig struct {
	// Listen addresses
	Listen    string `hcl:"listen,optional" json:"listen,omitempty"`         // HTTP listen address (default :80)
	TLSListen string `hcl:"tls_listen,optional" json:"tls_listen,omitempty"` // HTTPS listen address (default :443)

	// TLS Configuration
	TLSCert string `hcl:"tls_cert,optional" json:"tls_cert,omitempty"` // Path to TLS certificate
	TLSKey  string `hcl:"tls_key,optional" json:"tls_key,omitempty"`   // Path to TLS key

	// Behavior
	DisableRedirect bool `hcl:"disable_redirect,optional" json:"disable_redirect,omitempty" tui:"title=Disable Redirect,desc=Disable HTTP to HTTPS redirect"`
	ServeUI         bool `hcl:"serve_ui,optional" json:"serve_ui,omitempty" tui:"title=Enable UI,desc=Serve the web dashboard"`
	ServeAPI        bool `hcl:"serve_api,optional" json:"serve_api,omitempty" tui:"title=Enable API Routes,desc=Serve API on same port"`

	// Access Control
	Allow []AccessRule `hcl:"allow,block" json:"allow,omitempty"`
	Deny  []AccessRule `hcl:"deny,block" json:"deny,omitempty"`
}

// AccessRule defines criteria for allowing or denying access.
type AccessRule struct {
	// Single value fields
	Interface string `hcl:"interface,optional" json:"interface,omitempty"`
	Source    string `hcl:"source,optional" json:"source,omitempty"`

	// List value fields (for brevity)
	Interfaces []string `hcl:"interfaces,optional" json:"interfaces,omitempty"`
	Sources    []string `hcl:"sources,optional" json:"sources,omitempty"`
}
