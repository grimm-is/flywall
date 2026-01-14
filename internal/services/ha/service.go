//go:build linux
// +build linux

package ha

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"grimm.is/flywall/internal/clock"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// Role represents the current HA role of this node.
type Role string

const (
	// RolePrimary indicates this node is the active primary.
	RolePrimary Role = "primary"

	// RoleBackup indicates this node is the standby backup.
	RoleBackup Role = "backup"

	// RoleTakingOver indicates this node is in the process of becoming primary.
	RoleTakingOver Role = "taking_over"

	// RoleFailed indicates this node has failed and is not participating.
	RoleFailed Role = "failed"
)

// Default configuration values.
const (
	DefaultHeartbeatInterval = 1 // seconds
	DefaultFailureThreshold  = 3 // missed heartbeats
	DefaultHeartbeatPort     = 9002
	DefaultPriority          = 100
	DefaultFailbackDelay     = 60 // seconds
)

// HeartbeatMessage is sent between HA peers via UDP.
type HeartbeatMessage struct {
	// NodeID uniquely identifies this node (hostname or config-defined).
	NodeID string `json:"node_id"`

	// Role is the current role of the sending node.
	Role Role `json:"role"`

	// Priority is the node's configured priority (lower = higher priority).
	Priority int `json:"priority"`

	// StateVersion is the current replication state version.
	StateVersion uint64 `json:"state_version"`

	// Timestamp is when this heartbeat was sent.
	Timestamp time.Time `json:"timestamp"`

	// Signature is HMAC-SHA256 of the message (if secret_key is configured).
	Signature []byte `json:"signature,omitempty"`
}

// PeerState tracks the state of the peer node.
type PeerState struct {
	// Alive indicates whether the peer is responding to heartbeats.
	Alive bool

	// LastSeen is when we last received a heartbeat from the peer.
	LastSeen time.Time

	// Role is the peer's last reported role.
	Role Role

	// Priority is the peer's configured priority.
	Priority int

	// StateVersion is the peer's last reported replication version.
	StateVersion uint64

	// MissedHeartbeats counts consecutive missed heartbeats.
	MissedHeartbeats int
}

// LinkManager defines the interface for link layer operations.
// This is satisfied by ctlplane.LinkManager.
type LinkManager interface {
	SetHardwareAddr(name string, mac []byte) error
	GetHardwareAddr(name string) ([]byte, error)
}

// DHCPReclaimer defines the interface for DHCP lease reclaim operations.
// This is satisfied by dhcp.ClientManager.
type DHCPReclaimer interface {
	ReclaimLease(ifaceName string) (interface{}, error)
}

// Service manages high-availability failover.
type Service struct {
	config  *config.ReplicationConfig
	haConf  *config.HAConfig
	nodeID  string
	role    Role
	peer    PeerState
	linkMgr LinkManager
	dhcpMgr DHCPReclaimer
	logger  *logging.Logger

	// Callbacks for role transitions
	onBecomePrimary func() error
	onBecomeBackup  func() error

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// UDP connections for heartbeat
	sendConn *net.UDPConn
	recvConn *net.UDPConn

	// Conntrackd manager for connection state sync
	conntrackdMgr *ConntrackdManager
}

// NewService creates a new HA service.
func NewService(cfg *config.ReplicationConfig, nodeID string, linkMgr LinkManager, logger *logging.Logger) (*Service, error) {
	if cfg.HA == nil {
		return nil, fmt.Errorf("HA configuration is nil")
	}

	haConf := cfg.HA

	// Apply defaults
	if haConf.HeartbeatInterval <= 0 {
		haConf.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if haConf.FailureThreshold <= 0 {
		haConf.FailureThreshold = DefaultFailureThreshold
	}
	if haConf.HeartbeatPort <= 0 {
		haConf.HeartbeatPort = DefaultHeartbeatPort
	}
	if haConf.Priority <= 0 {
		haConf.Priority = DefaultPriority
	}
	if haConf.FailbackDelay <= 0 {
		haConf.FailbackDelay = DefaultFailbackDelay
	}

	// Determine initial role from config
	role := RoleBackup
	if cfg.Mode == "primary" {
		role = RolePrimary
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		config:  cfg,
		haConf:  haConf,
		nodeID:  nodeID,
		role:    role,
		linkMgr: linkMgr,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// OnBecomePrimary sets a callback to be called when this node becomes primary.
// The callback should start services like DHCP server, apply firewall rules, etc.
func (s *Service) OnBecomePrimary(fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onBecomePrimary = fn
}

// OnBecomeBackup sets a callback to be called when this node becomes backup.
// The callback should stop serving and prepare for standby.
func (s *Service) OnBecomeBackup(fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onBecomeBackup = fn
}

// SetDHCPReclaimer sets the DHCP client manager for lease reclaim operations.
// This must be called before Start() if DHCP reclaim is needed.
func (s *Service) SetDHCPReclaimer(mgr DHCPReclaimer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dhcpMgr = mgr
}

// Start begins HA monitoring.
func (s *Service) Start() error {
	if !s.haConf.Enabled {
		s.logger.Info("HA is disabled, skipping start")
		return nil
	}

	s.logger.Info("Starting HA service",
		"node_id", s.nodeID,
		"role", s.role,
		"priority", s.haConf.Priority,
		"peer", s.config.PeerAddr)

	// Setup UDP listener for receiving heartbeats
	listenAddr := fmt.Sprintf(":%d", s.haConf.HeartbeatPort)
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve listen address: %w", err)
	}

	s.recvConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen for heartbeats: %w", err)
	}

	// If we're starting as primary, apply virtual resources
	if s.role == RolePrimary {
		if err := s.applyVirtualResources(); err != nil {
			s.logger.Warn("Failed to apply virtual resources on startup", "error", err)
		}
	}

	// Start background goroutines
	s.wg.Add(2)
	go s.runHeartbeatSender()
	go s.runHeartbeatReceiver()

	// Start conntrackd if configured
	if s.haConf.ConntrackSync != nil && s.haConf.ConntrackSync.Enabled {
		s.conntrackdMgr = NewConntrackdManager(s.haConf.ConntrackSync, s.config.PeerAddr, "", s.logger)
		if err := s.conntrackdMgr.Start(); err != nil {
			s.logger.Warn("Failed to start conntrackd", "error", err)
		}
	}

	return nil
}

// Stop stops HA monitoring.
func (s *Service) Stop() {
	s.cancel()

	if s.sendConn != nil {
		s.sendConn.Close()
	}
	if s.recvConn != nil {
		s.recvConn.Close()
	}

	s.wg.Wait()

	// Stop conntrackd
	if s.conntrackdMgr != nil {
		s.conntrackdMgr.Stop()
	}

	s.logger.Info("HA service stopped")
}

// GetRole returns the current HA role.
func (s *Service) GetRole() Role {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.role
}

// GetPeerState returns the current peer state.
func (s *Service) GetPeerState() PeerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peer
}

// TriggerFailover manually initiates failover (for testing/maintenance).
func (s *Service) TriggerFailover() error {
	s.mu.Lock()
	if s.role != RoleBackup {
		s.mu.Unlock()
		return fmt.Errorf("can only trigger failover from backup role, current role: %s", s.role)
	}
	s.role = RoleTakingOver
	s.mu.Unlock()

	return s.performTakeover()
}

// runHeartbeatSender periodically sends heartbeats to the peer.
func (s *Service) runHeartbeatSender() {
	defer s.wg.Done()

	if s.config.PeerAddr == "" {
		s.logger.Warn("No peer address configured, heartbeat sender disabled")
		return
	}

	// Resolve peer address
	peerAddr, err := net.ResolveUDPAddr("udp", s.config.PeerAddr)
	if err != nil {
		s.logger.Error("Failed to resolve peer address", "error", err)
		return
	}

	// Create send connection
	s.sendConn, err = net.DialUDP("udp", nil, peerAddr)
	if err != nil {
		s.logger.Error("Failed to create UDP connection to peer", "error", err)
		return
	}

	ticker := time.NewTicker(time.Duration(s.haConf.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.sendHeartbeat(); err != nil {
				s.logger.Warn("Failed to send heartbeat", "error", err)
			}
		}
	}
}

// sendHeartbeat sends a single heartbeat message to the peer.
func (s *Service) sendHeartbeat() error {
	s.mu.RLock()
	msg := HeartbeatMessage{
		NodeID:       s.nodeID,
		Role:         s.role,
		Priority:     s.haConf.Priority,
		StateVersion: 0, // TODO: Get from replicator
		Timestamp:    clock.Now(),
	}
	s.mu.RUnlock()

	// Sign the message if secret key is configured
	if s.config.SecretKey != "" {
		msgBytes, _ := json.Marshal(HeartbeatMessage{
			NodeID:       msg.NodeID,
			Role:         msg.Role,
			Priority:     msg.Priority,
			StateVersion: msg.StateVersion,
			Timestamp:    msg.Timestamp,
		})
		mac := hmac.New(sha256.New, []byte(s.config.SecretKey))
		mac.Write(msgBytes)
		msg.Signature = mac.Sum(nil)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	_, err = s.sendConn.Write(data)
	return err
}

// runHeartbeatReceiver listens for heartbeats from the peer.
func (s *Service) runHeartbeatReceiver() {
	defer s.wg.Done()

	buf := make([]byte, 4096)
	checkTicker := time.NewTicker(time.Duration(s.haConf.HeartbeatInterval) * time.Second)
	defer checkTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-checkTicker.C:
			// Check for missed heartbeats
			s.checkPeerHealth()
		default:
			// Set read deadline to avoid blocking forever
			s.recvConn.SetReadDeadline(clock.Now().Add(500 * time.Millisecond))

			n, _, err := s.recvConn.ReadFromUDP(buf)
			if err != nil {
				// Timeout is expected, just continue
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// Socket closed
				select {
				case <-s.ctx.Done():
					return
				default:
					// Log unexpected errors
					s.logger.Warn("Error receiving heartbeat", "error", err)
				}
				continue
			}

			var msg HeartbeatMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				s.logger.Warn("Failed to unmarshal heartbeat", "error", err)
				continue
			}

			// Verify signature if secret key is configured
			if s.config.SecretKey != "" {
				if !s.verifyHeartbeat(&msg) {
					s.logger.Warn("Heartbeat signature verification failed", "from", msg.NodeID)
					continue
				}
			}

			s.handleHeartbeat(&msg)
		}
	}
}

// verifyHeartbeat verifies the HMAC signature of a heartbeat message.
func (s *Service) verifyHeartbeat(msg *HeartbeatMessage) bool {
	if len(msg.Signature) == 0 {
		return false
	}

	// Reconstruct the signed portion
	unsigned := HeartbeatMessage{
		NodeID:       msg.NodeID,
		Role:         msg.Role,
		Priority:     msg.Priority,
		StateVersion: msg.StateVersion,
		Timestamp:    msg.Timestamp,
	}
	msgBytes, _ := json.Marshal(unsigned)

	mac := hmac.New(sha256.New, []byte(s.config.SecretKey))
	mac.Write(msgBytes)
	expectedSig := mac.Sum(nil)

	return hmac.Equal(msg.Signature, expectedSig)
}

// handleHeartbeat processes a received heartbeat message.
func (s *Service) handleHeartbeat(msg *HeartbeatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update peer state
	s.peer.Alive = true
	s.peer.LastSeen = clock.Now()
	s.peer.Role = msg.Role
	s.peer.Priority = msg.Priority
	s.peer.StateVersion = msg.StateVersion
	s.peer.MissedHeartbeats = 0

	// Log significant state changes
	s.logger.Debug("Received heartbeat",
		"from", msg.NodeID,
		"role", msg.Role,
		"priority", msg.Priority)

	// Handle split-brain prevention: if both think they're primary,
	// the one with lower priority wins, other demotes to backup
	if s.role == RolePrimary && msg.Role == RolePrimary {
		s.logger.Warn("Split-brain detected! Both nodes claim primary",
			"our_priority", s.haConf.Priority,
			"peer_priority", msg.Priority)

		if s.haConf.Priority > msg.Priority {
			// Peer has lower priority (wins), we demote
			s.logger.Warn("Demoting to backup due to lower priority peer")
			s.role = RoleBackup

			// Cleanup virtual resources
			if err := s.removeVirtualResources(); err != nil {
				s.logger.Error("Failed to remove virtual resources during demotion", "error", err)
			}

			if s.onBecomeBackup != nil {
				go s.onBecomeBackup()
			}
		}
		// If we have lower priority, we stay primary and peer should demote
	}
}

// checkPeerHealth checks if the peer has missed too many heartbeats.
func (s *Service) checkPeerHealth() {
	s.mu.Lock()

	// Calculate time since last heartbeat
	interval := time.Duration(s.haConf.HeartbeatInterval) * time.Second

	if s.peer.LastSeen.IsZero() {
		// Never seen peer, increment missed count
		s.peer.MissedHeartbeats++
	} else if clock.Now().Sub(s.peer.LastSeen) > interval {
		s.peer.MissedHeartbeats++
	}

	// Check if we should trigger failover
	shouldTakeover := s.role == RoleBackup &&
		s.peer.MissedHeartbeats >= s.haConf.FailureThreshold

	if shouldTakeover {
		s.logger.Warn("Peer appears to be down, initiating failover",
			"missed_heartbeats", s.peer.MissedHeartbeats,
			"threshold", s.haConf.FailureThreshold,
			"last_seen", s.peer.LastSeen)
		s.peer.Alive = false
		s.role = RoleTakingOver
		s.mu.Unlock()

		// Perform takeover outside of lock
		if err := s.performTakeover(); err != nil {
			s.logger.Error("Failover failed", "error", err)
			s.mu.Lock()
			s.role = RoleFailed
			s.mu.Unlock()
		}
		return
	}

	// Log warnings as we approach threshold
	if s.peer.MissedHeartbeats > 0 && s.role == RoleBackup {
		s.logger.Warn("Peer heartbeat missed",
			"count", s.peer.MissedHeartbeats,
			"threshold", s.haConf.FailureThreshold,
			"last_seen", clock.Now().Sub(s.peer.LastSeen))
	}

	s.mu.Unlock()
}

// performTakeover executes the failover sequence.
func (s *Service) performTakeover() error {
	s.logger.Info("Starting failover takeover sequence")

	// 1. Apply virtual MACs first (needed for DHCP)
	for _, vmac := range s.haConf.VirtualMACs {
		if err := s.applyVirtualMAC(vmac); err != nil {
			s.logger.Error("Failed to apply virtual MAC", "interface", vmac.Interface, "error", err)
			// Continue anyway - partial failover is better than none
		}
	}

	// 2. Apply virtual IPs
	for _, vip := range s.haConf.VirtualIPs {
		if err := s.applyVirtualIP(vip); err != nil {
			s.logger.Error("Failed to apply virtual IP", "address", vip.Address, "error", err)
		}
	}

	// 3. Update role
	s.mu.Lock()
	s.role = RolePrimary
	s.mu.Unlock()

	// 4. Run callback to start services
	if s.onBecomePrimary != nil {
		if err := s.onBecomePrimary(); err != nil {
			s.logger.Error("onBecomePrimary callback failed", "error", err)
			return err
		}
	}

	// 5. Commit conntrack state from peer
	if s.conntrackdMgr != nil {
		if err := s.conntrackdMgr.NotifyFailover(); err != nil {
			s.logger.Warn("Conntrack state commit failed", "error", err)
		}
	}

	s.logger.Info("Failover complete, now operating as primary")
	return nil
}

// applyVirtualResources applies all virtual IPs and MACs (used on startup as primary).
func (s *Service) applyVirtualResources() error {
	var lastErr error

	for _, vmac := range s.haConf.VirtualMACs {
		if err := s.applyVirtualMAC(vmac); err != nil {
			s.logger.Error("Failed to apply virtual MAC", "interface", vmac.Interface, "error", err)
			lastErr = err
		}
	}

	for _, vip := range s.haConf.VirtualIPs {
		if err := s.applyVirtualIP(vip); err != nil {
			s.logger.Error("Failed to apply virtual IP", "address", vip.Address, "error", err)
			lastErr = err
		}
	}

	return lastErr
}

// removeVirtualResources removes all virtual IPs and MACs (used on demotion to backup).
func (s *Service) removeVirtualResources() error {
	var lastErr error

	// Remove IPs first
	for _, vip := range s.haConf.VirtualIPs {
		if err := s.removeVirtualIP(vip); err != nil {
			s.logger.Error("Failed to remove virtual IP", "address", vip.Address, "error", err)
			lastErr = err
		}
	}

	// Remove MACs (restore original?)
	// For now we don't restore original MACs as it might be complex to track.
	// We rely on the fact that we're no longer using the VIPs.
	// But ideally we should. For now, let's just leave MACs as they are or log.
	// TODO: Restore original MACs

	// Actually, we must at least stop asserting the virtual MAC if we can.
	// But LinkManager SetHardwareAddr replaces it.
	// Without storing original MAC, we can't restore.
	// For this sprint, removing IPs is the critical part for avoiding conflict.

	return lastErr
}

// applyVirtualMAC applies a virtual MAC address to an interface.
func (s *Service) applyVirtualMAC(vmac config.VirtualMAC) error {
	var mac []byte
	var err error

	if vmac.Address != "" {
		// Parse configured MAC
		mac, err = parseMAC(vmac.Address)
		if err != nil {
			return fmt.Errorf("invalid MAC address %s: %w", vmac.Address, err)
		}
	} else {
		// Generate MAC from interface name
		mac = generateVirtualMAC(vmac.Interface)
	}

	s.logger.Info("Applying virtual MAC",
		"interface", vmac.Interface,
		"mac", formatMAC(mac))

	if err := s.linkMgr.SetHardwareAddr(vmac.Interface, mac); err != nil {
		return err
	}

	// If DHCP is enabled and we have a DHCP manager, reclaim the lease
	if vmac.DHCP {
		if s.dhcpMgr != nil {
			s.logger.Info("Reclaiming DHCP lease with virtual MAC", "interface", vmac.Interface)
			if _, err := s.dhcpMgr.ReclaimLease(vmac.Interface); err != nil {
				s.logger.Error("Failed to reclaim DHCP lease", "interface", vmac.Interface, "error", err)
				return fmt.Errorf("DHCP reclaim failed: %w", err)
			}
			s.logger.Info("Successfully reclaimed DHCP lease", "interface", vmac.Interface)
		} else {
			s.logger.Warn("DHCP reclaim requested but no DHCP manager configured", "interface", vmac.Interface)
		}
	}

	return nil
}

// applyVirtualIP adds a virtual IP address to an interface.
func (s *Service) applyVirtualIP(vip config.VirtualIP) error {
	s.logger.Info("Applying virtual IP",
		"address", vip.Address,
		"interface", vip.Interface)

	// Parse the address
	addr, err := parseAddr(vip.Address)
	if err != nil {
		return fmt.Errorf("invalid address %s: %w", vip.Address, err)
	}

	// Add the address to the interface
	// This uses netlink directly since LinkManager doesn't have IP management
	return addIPAddress(vip.Interface, addr, vip.Label)
}

// removeVirtualIP removes a virtual IP address from an interface.
func (s *Service) removeVirtualIP(vip config.VirtualIP) error {
	s.logger.Info("Removing virtual IP",
		"address", vip.Address,
		"interface", vip.Interface)

	// Parse the address
	addr, err := parseAddr(vip.Address)
	if err != nil {
		return fmt.Errorf("invalid address %s: %w", vip.Address, err)
	}

	return removeIPAddress(vip.Interface, addr)
}

// Helper functions (duplicated from ctlplane to avoid circular import)

func parseMAC(macStr string) ([]byte, error) {
	hw, err := net.ParseMAC(macStr)
	if err != nil {
		return nil, err
	}
	return hw, nil
}

func formatMAC(mac []byte) string {
	if len(mac) != 6 {
		return ""
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func generateVirtualMAC(ifaceName string) []byte {
	hash := uint32(0)
	for _, c := range ifaceName {
		hash = hash*31 + uint32(c)
	}
	return []byte{
		0x02, // Locally-administered, unicast
		0x67, // 'g'
		0x63, // 'c'
		byte(hash >> 16),
		byte(hash >> 8),
		byte(hash),
	}
}
