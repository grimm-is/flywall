// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package main

import (
	"os"
	"time"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
	"grimm.is/flywall/internal/i18n"
	"grimm.is/flywall/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

var Printer = i18n.NewCLIPrinter()

// MockBackend implements tui.Backend for testing
type MockBackend struct{}

func (m *MockBackend) GetStatus() (*tui.EnrichedStatus, error) {
	return &tui.EnrichedStatus{
		Running: true,
		Uptime:  "3d 14h 22m",
	}, nil
}

func (m *MockBackend) GetSystemStats() (*ctlplane.SystemStats, error) {
	return &ctlplane.SystemStats{
		CPUUsage:    15.5,
		MemoryTotal: 8 * 1024 * 1024 * 1024,
		MemoryUsed:  2 * 1024 * 1024 * 1024,
	}, nil
}

func (m *MockBackend) GetFlows(filter string) ([]tui.Flow, error) {
	return []tui.Flow{
		{Proto: "tcp", Src: "10.0.0.5:12345", Dst: "1.1.1.1:443", State: "ESTABLISHED"},
		{Proto: "udp", Src: "10.0.0.5:53", Dst: "8.8.8.8:53", State: "UNREPLIED"},
	}, nil
}

func (m *MockBackend) GetConfig() (*config.Config, error) {
	cfg := &config.Config{
		SchemaVersion: "1.0",
		API: &config.APIConfig{
			Enabled:             true,
			Listen:              ":8080",
			DisableHTTPRedirect: false,
		},
		Features: &config.Features{
			IntegrityMonitoring: true,
		},
	}
	return cfg, nil
}

func (m *MockBackend) ApplyConfig(cfg *config.Config) error {
	return nil
}

func (m *MockBackend) ListBackups() ([]ctlplane.BackupInfo, error) {
	return []ctlplane.BackupInfo{
		{Version: 1, Description: "Initial Setup", Timestamp: time.Now().Add(-2 * time.Hour).Format(time.RFC3339)},
	}, nil
}

func (m *MockBackend) RestoreBackup(version int) error {
	return nil
}

func (m *MockBackend) Reboot() error {
	return nil
}

func (m *MockBackend) GetBandwidth(window string) ([]ctlplane.BandwidthPoint, error) {
	return []ctlplane.BandwidthPoint{}, nil
}

func (m *MockBackend) GetAlerts(limit int) ([]alerting.AlertEvent, error) {
	return []alerting.AlertEvent{}, nil
}

func main() {
	Printer.Printf("Starting %s TUI Demo...\n", brand.Name)
	Printer.Println("Verifying new components: Card, Form, Tabs, Alert")
	time.Sleep(1 * time.Second) // Give user time to see message

	backend := &MockBackend{}
	// Make sure internal/tui/app.go imports are correct and public
	p := tea.NewProgram(tui.NewModel(backend), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		Printer.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
