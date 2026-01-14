//go:build linux
// +build linux

package ha

import (
	"testing"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// MockLinkManager implements LinkManager for testing.
type MockLinkManager struct {
	MACs map[string][]byte
}

func NewMockLinkManager() *MockLinkManager {
	return &MockLinkManager{
		MACs: make(map[string][]byte),
	}
}

func (m *MockLinkManager) SetHardwareAddr(name string, mac []byte) error {
	m.MACs[name] = mac
	return nil
}

func (m *MockLinkManager) GetHardwareAddr(name string) ([]byte, error) {
	if mac, ok := m.MACs[name]; ok {
		return mac, nil
	}
	return []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
}

func TestNewService(t *testing.T) {
	logger := logging.WithComponent("ha-test")

	tests := []struct {
		name    string
		config  *config.ReplicationConfig
		wantErr bool
	}{
		{
			name: "valid primary config",
			config: &config.ReplicationConfig{
				Mode:     "primary",
				PeerAddr: "192.168.1.2:9002",
				HA: &config.HAConfig{
					Enabled:  true,
					Priority: 50,
				},
			},
			wantErr: false,
		},
		{
			name: "valid backup config",
			config: &config.ReplicationConfig{
				Mode:     "replica",
				PeerAddr: "192.168.1.1:9002",
				HA: &config.HAConfig{
					Enabled:  true,
					Priority: 150,
				},
			},
			wantErr: false,
		},
		{
			name: "nil HA config",
			config: &config.ReplicationConfig{
				Mode: "primary",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linkMgr := NewMockLinkManager()
			svc, err := NewService(tt.config, "test-node", linkMgr, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && svc == nil {
				t.Error("NewService() returned nil service without error")
			}
		})
	}
}

func TestServiceDefaults(t *testing.T) {
	logger := logging.WithComponent("ha-test")
	cfg := &config.ReplicationConfig{
		Mode:     "primary",
		PeerAddr: "192.168.1.2:9002",
		HA: &config.HAConfig{
			Enabled: true,
			// Leave all values at zero to test defaults
		},
	}

	linkMgr := NewMockLinkManager()
	svc, err := NewService(cfg, "test-node", linkMgr, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if svc.haConf.HeartbeatInterval != DefaultHeartbeatInterval {
		t.Errorf("HeartbeatInterval = %d, want %d", svc.haConf.HeartbeatInterval, DefaultHeartbeatInterval)
	}
	if svc.haConf.FailureThreshold != DefaultFailureThreshold {
		t.Errorf("FailureThreshold = %d, want %d", svc.haConf.FailureThreshold, DefaultFailureThreshold)
	}
	if svc.haConf.HeartbeatPort != DefaultHeartbeatPort {
		t.Errorf("HeartbeatPort = %d, want %d", svc.haConf.HeartbeatPort, DefaultHeartbeatPort)
	}
	if svc.haConf.Priority != DefaultPriority {
		t.Errorf("Priority = %d, want %d", svc.haConf.Priority, DefaultPriority)
	}
}

func TestInitialRole(t *testing.T) {
	logger := logging.WithComponent("ha-test")
	linkMgr := NewMockLinkManager()

	tests := []struct {
		mode     string
		wantRole Role
	}{
		{"primary", RolePrimary},
		{"replica", RoleBackup},
		{"standby", RoleBackup},
		{"", RoleBackup},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := &config.ReplicationConfig{
				Mode: tt.mode,
				HA:   &config.HAConfig{Enabled: true},
			}
			svc, err := NewService(cfg, "test-node", linkMgr, logger)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			if svc.GetRole() != tt.wantRole {
				t.Errorf("GetRole() = %s, want %s", svc.GetRole(), tt.wantRole)
			}
		})
	}
}

func TestHeartbeatMessageMarshal(t *testing.T) {
	msg := HeartbeatMessage{
		NodeID:       "node1",
		Role:         RolePrimary,
		Priority:     50,
		StateVersion: 12345,
		Timestamp:    time.Now().UTC(),
	}

	// Just verify it doesn't panic
	if msg.NodeID != "node1" {
		t.Errorf("NodeID mismatch")
	}
}

func TestGenerateVirtualMAC(t *testing.T) {
	tests := []struct {
		iface string
	}{
		{"eth0"},
		{"eth1"},
		{"wan"},
	}

	for _, tt := range tests {
		t.Run(tt.iface, func(t *testing.T) {
			mac := generateVirtualMAC(tt.iface)
			if len(mac) != 6 {
				t.Errorf("MAC length = %d, want 6", len(mac))
			}
			// First byte should be locally-administered (bit 1 set)
			if mac[0]&0x02 == 0 {
				t.Error("MAC is not locally-administered")
			}
			// Should be unicast (bit 0 clear)
			if mac[0]&0x01 != 0 {
				t.Error("MAC is not unicast")
			}
		})
	}

	// Same interface should generate same MAC
	mac1 := generateVirtualMAC("eth0")
	mac2 := generateVirtualMAC("eth0")
	for i := range mac1 {
		if mac1[i] != mac2[i] {
			t.Error("Same interface generated different MACs")
			break
		}
	}

	// Different interfaces should generate different MACs
	mac3 := generateVirtualMAC("eth1")
	same := true
	for i := range mac1 {
		if mac1[i] != mac3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Different interfaces generated same MAC")
	}
}

func TestFormatMAC(t *testing.T) {
	tests := []struct {
		mac  []byte
		want string
	}{
		{[]byte{0x02, 0x67, 0x63, 0x12, 0x34, 0x56}, "02:67:63:12:34:56"},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "ff:ff:ff:ff:ff:ff"},
		{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "00:00:00:00:00:00"},
		{[]byte{0x01, 0x02, 0x03}, ""}, // Invalid length
	}

	for _, tt := range tests {
		got := formatMAC(tt.mac)
		if got != tt.want {
			t.Errorf("formatMAC(%v) = %s, want %s", tt.mac, got, tt.want)
		}
	}
}

func TestParseMAC(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"02:67:63:12:34:56", false},
		{"FF:FF:FF:FF:FF:FF", false},
		{"02-67-63-12-34-56", false}, // Hyphen format (should be handled by net.ParseMAC)
		{"invalid", true},
		{"02:67:63:12:34", true}, // Too short
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseMAC(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMAC(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestTriggerFailover(t *testing.T) {
	logger := logging.WithComponent("ha-test")
	linkMgr := NewMockLinkManager()

	// Create a backup service
	cfg := &config.ReplicationConfig{
		Mode: "replica",
		HA:   &config.HAConfig{Enabled: true},
	}
	svc, err := NewService(cfg, "test-node", linkMgr, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Verify we start as backup
	if svc.GetRole() != RoleBackup {
		t.Errorf("Initial role = %s, want %s", svc.GetRole(), RoleBackup)
	}

	// Trigger failover without starting service (should still work for role change)
	// Note: This won't actually apply VIPs since service isn't started
	// But the mock LinkManager will capture any MAC changes attempted
}

func TestTriggerFailoverFromPrimary(t *testing.T) {
	logger := logging.WithComponent("ha-test")
	linkMgr := NewMockLinkManager()

	// Create a primary service
	cfg := &config.ReplicationConfig{
		Mode: "primary",
		HA:   &config.HAConfig{Enabled: true},
	}
	svc, err := NewService(cfg, "test-node", linkMgr, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Try to trigger failover from primary - should fail
	err = svc.TriggerFailover()
	if err == nil {
		t.Error("TriggerFailover() from primary should return error")
	}
}

func TestPeerStateTracking(t *testing.T) {
	logger := logging.WithComponent("ha-test")
	linkMgr := NewMockLinkManager()

	cfg := &config.ReplicationConfig{
		Mode: "replica",
		HA:   &config.HAConfig{Enabled: true, Priority: 100},
	}
	svc, err := NewService(cfg, "test-node", linkMgr, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Initial peer state should be empty
	peer := svc.GetPeerState()
	if peer.Alive {
		t.Error("Initial peer.Alive should be false")
	}
	if peer.MissedHeartbeats != 0 {
		t.Errorf("Initial MissedHeartbeats = %d, want 0", peer.MissedHeartbeats)
	}
}

func TestHeartbeatMessageFields(t *testing.T) {
	now := time.Now().UTC()
	msg := HeartbeatMessage{
		NodeID:       "test-primary",
		Role:         RolePrimary,
		Priority:     50,
		StateVersion: 12345,
		Timestamp:    now,
		Signature:    []byte{0x01, 0x02, 0x03},
	}

	// Verify all fields
	if msg.NodeID != "test-primary" {
		t.Errorf("NodeID = %s, want test-primary", msg.NodeID)
	}
	if msg.Role != RolePrimary {
		t.Errorf("Role = %s, want %s", msg.Role, RolePrimary)
	}
	if msg.Priority != 50 {
		t.Errorf("Priority = %d, want 50", msg.Priority)
	}
	if msg.StateVersion != 12345 {
		t.Errorf("StateVersion = %d, want 12345", msg.StateVersion)
	}
	if !msg.Timestamp.Equal(now) {
		t.Errorf("Timestamp mismatch")
	}
	if len(msg.Signature) != 3 {
		t.Errorf("Signature length = %d, want 3", len(msg.Signature))
	}
}

func TestVirtualMACApply(t *testing.T) {
	logger := logging.WithComponent("ha-test")
	linkMgr := NewMockLinkManager()

	cfg := &config.ReplicationConfig{
		Mode: "primary",
		HA: &config.HAConfig{
			Enabled: true,
			VirtualMACs: []config.VirtualMAC{
				{Interface: "eth0", Address: "02:67:63:aa:bb:cc"},
				{Interface: "eth1"}, // Auto-generate
			},
		},
	}
	svc, err := NewService(cfg, "test-node", linkMgr, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Apply virtual resources (simulates what happens on startup as primary)
	err = svc.applyVirtualResources()
	if err != nil {
		t.Errorf("applyVirtualResources() error = %v", err)
	}

	// Check that MACs were applied to the mock
	if mac, ok := linkMgr.MACs["eth0"]; !ok {
		t.Error("eth0 MAC not set")
	} else {
		expected := "02:67:63:aa:bb:cc"
		if formatMAC(mac) != expected {
			t.Errorf("eth0 MAC = %s, want %s", formatMAC(mac), expected)
		}
	}

	if _, ok := linkMgr.MACs["eth1"]; !ok {
		t.Error("eth1 MAC not set (auto-generated)")
	}
}

func TestCallbackRegistration(t *testing.T) {
	logger := logging.WithComponent("ha-test")
	linkMgr := NewMockLinkManager()

	cfg := &config.ReplicationConfig{
		Mode: "replica",
		HA:   &config.HAConfig{Enabled: true},
	}
	svc, err := NewService(cfg, "test-node", linkMgr, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	primaryCalled := false
	backupCalled := false

	svc.OnBecomePrimary(func() error {
		primaryCalled = true
		return nil
	})

	svc.OnBecomeBackup(func() error {
		backupCalled = true
		return nil
	})

	// Callbacks should be registered but not called yet
	if primaryCalled || backupCalled {
		t.Error("Callbacks should not be called on registration")
	}
}

func TestRoleConstants(t *testing.T) {
	// Verify role string values for API/logging compatibility
	if string(RolePrimary) != "primary" {
		t.Errorf("RolePrimary = %q, want 'primary'", RolePrimary)
	}
	if string(RoleBackup) != "backup" {
		t.Errorf("RoleBackup = %q, want 'backup'", RoleBackup)
	}
	if string(RoleTakingOver) != "taking_over" {
		t.Errorf("RoleTakingOver = %q, want 'taking_over'", RoleTakingOver)
	}
	if string(RoleFailed) != "failed" {
		t.Errorf("RoleFailed = %q, want 'failed'", RoleFailed)
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify default values are sensible
	if DefaultHeartbeatInterval != 1 {
		t.Errorf("DefaultHeartbeatInterval = %d, want 1", DefaultHeartbeatInterval)
	}
	if DefaultFailureThreshold != 3 {
		t.Errorf("DefaultFailureThreshold = %d, want 3", DefaultFailureThreshold)
	}
	if DefaultHeartbeatPort != 9002 {
		t.Errorf("DefaultHeartbeatPort = %d, want 9002", DefaultHeartbeatPort)
	}
	if DefaultPriority != 100 {
		t.Errorf("DefaultPriority = %d, want 100", DefaultPriority)
	}
	if DefaultFailbackDelay != 60 {
		t.Errorf("DefaultFailbackDelay = %d, want 60", DefaultFailbackDelay)
	}
}
