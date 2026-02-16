// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui

import (
	"time"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
)

// Ensure Backend implementation
// Note: We use the direct client for now, but in future this might be a gRPC client
type LocalBackend struct {
	client *ctlplane.Client
}

func NewLocalBackend(client *ctlplane.Client) *LocalBackend {
	return &LocalBackend{client: client}
}

func (b *LocalBackend) GetStatus() (*EnrichedStatus, error) {
	status, err := b.client.GetStatus()
	if err != nil {
		return nil, err
	}
	return &EnrichedStatus{
		Running: status.Running,
		Uptime:  status.Uptime,
	}, nil
}

func (b *LocalBackend) GetSystemStats() (*ctlplane.SystemStats, error) {
	return b.client.GetSystemStats()
}

func (b *LocalBackend) GetFlows(filter string) ([]Flow, error) {
	// Use real client to fetch flows
	flows, _, err := b.client.GetFlows(filter, 100, 0)
	if err != nil {
		return nil, err
	}

	result := make([]Flow, len(flows))
	for i, f := range flows {
		// Use best hint (domain) if available, otherwise fallback to IP sample
		dest := f.BestHint
		if dest == "" {
			dest = f.DstIPSample
		}

		result[i] = Flow{
			ID:    f.ID,
			Proto: f.Protocol,
			Src:   f.SrcIP,
			Dst:   dest,
			State: string(f.State),
		}
	}
	return result, nil
}

func (b *LocalBackend) GetConfig() (*config.Config, error) {
	return b.client.GetConfig()
}

func (b *LocalBackend) ApplyConfig(cfg *config.Config) error {
	return b.client.ApplyConfig(cfg)
}

func (b *LocalBackend) ReloadConfig() error {
	return b.RestartService("firewall")
}

func (b *LocalBackend) ListBackups() ([]ctlplane.BackupInfo, error) {
	reply, err := b.client.ListBackups()
	if err != nil {
		return nil, err
	}
	return reply.Backups, nil
}

func (b *LocalBackend) RestoreBackup(version int) error {
	_, err := b.client.RestoreBackup(version)
	return err
}

func (b *LocalBackend) Reboot() error {
	return b.client.Reboot()
}

func (b *LocalBackend) RestartService(name string) error {
	return b.client.RestartService(name)
}

func (b *LocalBackend) GetBandwidth(window string) ([]ctlplane.BandwidthPoint, error) {
	// Parse window to duration/start-end
	// Default to last 1 hour
	to := time.Now()
	from := to.Add(-1 * time.Hour)

	if window == "24h" {
		from = to.Add(-24 * time.Hour)
	}

	return b.client.GetAnalyticsBandwidth(&ctlplane.GetAnalyticsBandwidthArgs{
		From: from,
		To:   to,
	})
}

func (b *LocalBackend) GetAlerts(limit int) ([]alerting.AlertEvent, error) {
	return b.client.GetAlertHistory(limit)
}

func (b *LocalBackend) ApproveFlow(id int64) error {
	return b.client.ApproveFlow(id)
}

func (b *LocalBackend) DenyFlow(id int64) error {
	return b.client.DenyFlow(id)
}

func (b *LocalBackend) GetServices() ([]ctlplane.ServiceStatus, error) {
	return b.client.GetServices()
}
