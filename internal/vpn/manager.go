// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package vpn

import (
	"context"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// Manager handles the lifecycle of all VPN providers.
type Manager struct {
	providers []Provider
	logger    *logging.Logger
}

// NewManager creates a new VPN manager from configuration.
func NewManager(cfg *config.VPNConfig, logger *logging.Logger) (*Manager, error) {
	m := &Manager{
		logger: logger,
	}

	if cfg == nil {
		return m, nil
	}

	// Initialize WireGuard providers
	for _, wgCfg := range cfg.WireGuard {
		if !wgCfg.Enabled {
			continue
		}

		// Default interface to Name if empty
		if wgCfg.Interface == "" {
			wgCfg.Interface = wgCfg.Name
		}

		// Map config to internal vpn type
		internalCfg := WireGuardConfig{
			Enabled:          wgCfg.Enabled,
			Interface:        wgCfg.Interface,
			ManagementAccess: wgCfg.ManagementAccess,
			Zone:             wgCfg.Zone,
			PrivateKey:       wgCfg.PrivateKey,
			PrivateKeyFile:   wgCfg.PrivateKeyFile,
			ListenPort:       wgCfg.ListenPort,
			Address:          wgCfg.Address,
			MTU:              wgCfg.MTU,
			FWMark:           wgCfg.FWMark,
			Table:            wgCfg.Table,
		}

		// Convert peers
		for _, peer := range wgCfg.Peers {
			internalPeer := WireGuardPeer{
				Name:                peer.Name,
				PublicKey:           peer.PublicKey,
				PresharedKey:        peer.PresharedKey,
				Endpoint:            peer.Endpoint,
				AllowedIPs:          peer.AllowedIPs,
				PersistentKeepalive: peer.PersistentKeepalive,
			}
			internalCfg.Peers = append(internalCfg.Peers, internalPeer)
		}

		provider := NewWireGuardManager(internalCfg, logger)
		m.providers = append(m.providers, provider)
	}

	// Initialize Tailscale providers
	for _, tsCfg := range cfg.Tailscale {
		if !tsCfg.Enabled {
			continue
		}

		// Default interface to Name if empty
		if tsCfg.Interface == "" {
			tsCfg.Interface = tsCfg.Name
		}

		internalCfg := TailscaleConfig{
			Enabled:           tsCfg.Enabled,
			Interface:         tsCfg.Interface,
			AuthKey:           string(tsCfg.AuthKey),
			AuthKeyEnv:        tsCfg.AuthKeyEnv,
			ControlURL:        tsCfg.ControlURL,
			ManagementAccess:  tsCfg.ManagementAccess,
			Zone:              tsCfg.Zone,
			AdvertiseRoutes:   tsCfg.AdvertiseRoutes,
			AcceptRoutes:      tsCfg.AcceptRoutes,
			AdvertiseExitNode: tsCfg.AdvertiseExitNode,
			ExitNode:          tsCfg.ExitNode,
		}

		provider := NewTailscaleManager(internalCfg, logger)
		m.providers = append(m.providers, provider)
	}

	return m, nil
}

// Start starts all managed VPN providers.
func (m *Manager) Start(ctx context.Context) error {
	for _, p := range m.providers {
		m.logger.Info("Starting VPN provider", "type", p.Type(), "interface", p.Interface())
		if err := p.Start(ctx); err != nil {
			m.logger.Warn("Failed to start VPN provider", "interface", p.Interface(), "error", err)
			// Continue trying others? Or fail?
			// For robustness, likely continue but log error.
		}
	}
	return nil
}

// Stop stops all managed VPN providers.
func (m *Manager) Stop() {
	for _, p := range m.providers {
		m.logger.Info("Stopping VPN provider", "interface", p.Interface())
		p.Stop()
	}
}
