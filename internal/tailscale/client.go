// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tailscale

import (
	"context"
	"fmt"

	"tailscale.com/client/local"
	"tailscale.com/ipn/ipnstate"
)

// Client wraps the tailscale local client
type Client struct {
	lc *local.Client
}

// NewClient creates a new Tailscale client
// It connects to the default socket location.
func NewClient() *Client {
	return &Client{
		lc: &local.Client{}, // Uses default paths
	}
}

// Status returns the current status of the Tailscale backend
func (c *Client) Status(ctx context.Context) (*ipnstate.Status, error) {
	return c.lc.Status(ctx)
}

// Up brings the network up (equivalent to 'tailscale up')
// Note: 'tailscale up' via API is complex as it involves prefs.
// For now, we interact via CLI execution or limited API.
// The localapi `EditPrefs` can be used to set WantRunning=true.
func (c *Client) Up(ctx context.Context) error {
	// Simple Up: Set WantRunning = true
	// Detailed configuration (auth key, etc) is harder via pure localAPI without interactive flow.
	// Often best to shell out for complex up, but we want API if possible.
	// Let's start with simple status toggle.

	st, err := c.lc.Status(ctx)
	if err != nil {
		return err
	}

	// We need to fetch current prefs to update them?
	// local.Client doesn't expose EditPrefs easily directly in older versions, checking symbols...
	// `Start` method exists?
	// `Start` is usually for the backend.

	// If we can't do it easily via Go client immediately without diving deep into ipn types,
	// we'll stick to Status/Whois for now and potentially shell out for 'up' commands in the CLI wrapper
	// or implement `EditPrefs` manual construction.

	// For this first pass, let's expose Status and basic introspection.
	// Actual "Up" command usually needs interactive login URL handling which is handled by the `tailscale up` CLI.
	// Implementing a full `tailscale up` equivalent in library code is non-trivial.
	// So `Up` here might strictly mean "Set WantRunning=true".

	_ = st
	return fmt.Errorf("not implemented via API yet, use CLI")
}
