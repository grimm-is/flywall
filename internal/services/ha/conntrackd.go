//go:build linux
// +build linux

package ha

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// ConntrackdManager manages the conntrackd daemon for connection state synchronization.
type ConntrackdManager struct {
	config    *config.ConntrackSyncConfig
	peerAddr  string
	syncIface string
	logger    *logging.Logger
	cmd       *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
}

// Default conntrackd settings
const (
	DefaultConntrackdPort           = 3780
	DefaultConntrackdMulticastGroup = "225.0.0.50"
)

// conntrackdConfigTemplate is the configuration template for conntrackd.
// It uses the FT-FW (Firewall) synchronization model with FTFW for reliable sync.
const conntrackdConfigTemplate = `#
# Flywall conntrackd configuration (auto-generated)
#

Sync {
	Mode FTFW {
		DisableExternalCache Off
		CommitTimeout 1800
		PurgeTimeout 5
	}

	{{if .Multicast}}
	Multicast {
		IPv4_address {{.MulticastGroup}}
		Group 3780
		IPv4_interface {{.SyncIP}}
		Interface {{.Interface}}
		SndSocketBuffer 1249280
		RcvSocketBuffer 1249280
		Checksum on
	}
	{{else}}
	UDP {
		IPv4_address {{.PeerIP}}
		IPv4_Destination_Address {{.PeerIP}}
		Port {{.Port}}
		Interface {{.Interface}}
		SndSocketBuffer 1249280
		RcvSocketBuffer 1249280
		Checksum on
	}
	{{end}}
}

General {
	Nice -20
	HashSize 32768
	HashLimit 131072
	LogFile /var/log/conntrackd.log
	Syslog on
	LockFile /var/lock/conntrackd.lock

	UNIX {
		Path /var/run/conntrackd.ctl
		Backlog 20
	}

	NetlinkBufferSize 2097152
	NetlinkBufferSizeMaxGrowth 8388608

	Filter From Userspace {
		Protocol Accept {
			TCP
			UDP
			ICMP
		}
		Address Ignore {
			IPv4_address 127.0.0.1
			IPv4_address {{.SyncIP}}
		}
	}
}
`

// NewConntrackdManager creates a new conntrackd manager.
func NewConntrackdManager(cfg *config.ConntrackSyncConfig, peerAddr, syncIface string, logger *logging.Logger) *ConntrackdManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConntrackdManager{
		config:    cfg,
		peerAddr:  peerAddr,
		syncIface: syncIface,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// GenerateConfig generates the conntrackd configuration file.
func (m *ConntrackdManager) GenerateConfig() (string, error) {
	// Determine sync interface
	iface := m.syncIface
	if m.config.Interface != "" {
		iface = m.config.Interface
	}

	// Get local IP on sync interface for filtering
	syncIP := getInterfaceIP(iface)
	if syncIP == "" {
		syncIP = "127.0.0.1" // Fallback
	}

	// Determine peer IP (strip port if present)
	peerIP := m.peerAddr
	if idx := lastIndexByte(peerIP, ':'); idx != -1 {
		peerIP = peerIP[:idx]
	}

	// Port
	port := m.config.Port
	if port == 0 {
		port = DefaultConntrackdPort
	}

	// Multicast vs Unicast
	useMulticast := m.config.MulticastGroup != ""
	multicastGroup := m.config.MulticastGroup
	if multicastGroup == "" && useMulticast {
		multicastGroup = DefaultConntrackdMulticastGroup
	}

	data := struct {
		Interface      string
		SyncIP         string
		PeerIP         string
		Port           int
		Multicast      bool
		MulticastGroup string
	}{
		Interface:      iface,
		SyncIP:         syncIP,
		PeerIP:         peerIP,
		Port:           port,
		Multicast:      useMulticast,
		MulticastGroup: multicastGroup,
	}

	tmpl, err := template.New("conntrackd").Parse(conntrackdConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Write to runtime directory
	runDir := filepath.Join(brand.GetRunDir(), "ha")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create run directory: %w", err)
	}

	configPath := filepath.Join(runDir, "conntrackd.conf")
	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	m.logger.Info("Generated conntrackd config", "path", configPath)
	return configPath, nil
}

// Start starts the conntrackd daemon.
func (m *ConntrackdManager) Start() error {
	if !m.config.Enabled {
		return nil
	}

	// Check if conntrackd is available
	if _, err := exec.LookPath("conntrackd"); err != nil {
		m.logger.Warn("conntrackd not found, conntrack sync disabled")
		return nil
	}

	configPath, err := m.GenerateConfig()
	if err != nil {
		return err
	}

	m.cmd = exec.CommandContext(m.ctx, "conntrackd", "-C", configPath, "-d")
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start conntrackd: %w", err)
	}

	m.logger.Info("Started conntrackd", "pid", m.cmd.Process.Pid)
	return nil
}

// Stop stops the conntrackd daemon.
func (m *ConntrackdManager) Stop() {
	m.cancel()

	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
	}

	// Also try pkill as backup
	exec.Command("pkill", "-9", "conntrackd").Run()

	m.logger.Info("Stopped conntrackd")
}

// NotifyFailover triggers bulk state injection after becoming primary.
// This commits the external cache (synced from peer) to the kernel.
func (m *ConntrackdManager) NotifyFailover() error {
	if !m.config.Enabled {
		return nil
	}

	// Wait briefly for any in-flight sync
	time.Sleep(100 * time.Millisecond)

	// Commit external cache to kernel conntrack table
	// -n: commit external cache
	// -B: commit external cache and flush internal cache
	cmd := exec.Command("conntrackd", "-n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("conntrackd commit failed: %w", err)
	}

	m.logger.Info("Committed conntrack state from peer")
	return nil
}

// NotifyPrimary signals that we're now primary and should start fresh internal cache.
func (m *ConntrackdManager) NotifyPrimary() error {
	if !m.config.Enabled {
		return nil
	}

	// Flush internal cache and start syncing our state to peer
	cmd := exec.Command("conntrackd", "-f", "internal")
	if err := cmd.Run(); err != nil {
		m.logger.Warn("Failed to flush internal cache", "error", err)
	}

	// Request resync from kernel
	cmd = exec.Command("conntrackd", "-R")
	if err := cmd.Run(); err != nil {
		m.logger.Warn("Failed to request resync", "error", err)
	}

	return nil
}

// Helper functions

func getInterfaceIP(name string) string {
	// Use ip command to get IPv4 address
	out, err := exec.Command("ip", "-4", "-o", "addr", "show", name).Output()
	if err != nil {
		return ""
	}
	// Parse "2: eth0    inet 192.168.1.1/24 ..."
	fields := bytes.Fields(out)
	for i, f := range fields {
		if string(f) == "inet" && i+1 < len(fields) {
			ip := string(fields[i+1])
			// Strip /prefix
			if idx := lastIndexByte(ip, '/'); idx != -1 {
				return ip[:idx]
			}
			return ip
		}
	}
	return ""
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
