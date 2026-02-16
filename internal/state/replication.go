// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"grimm.is/flywall/internal/clock"

	"grimm.is/flywall/internal/errors"
	"grimm.is/flywall/internal/logging"
)

// ReplicationMode defines the replication role.
type ReplicationMode string

const (
	ModePrimary ReplicationMode = "primary"
	ModeReplica ReplicationMode = "replica"
	ModeStandby ReplicationMode = "standby" // For upgrades
)

// ReplicationConfig configures the replication layer.
type ReplicationConfig struct {
	Mode           ReplicationMode
	ListenAddr     string        // For primary: where to accept replica connections
	PrimaryAddr    string        // For replica: where to connect to primary
	PeerAddr       string        // For high availability: address of the peer (for reverse sync)
	ReconnectDelay time.Duration // How long to wait before reconnecting
	SyncTimeout    time.Duration // Timeout for initial sync

	// Security settings
	SecretKey   string // PSK for HMAC authentication (required for secure mode)
	TLSCertFile string // Server/client certificate file
	TLSKeyFile  string // Server/client private key file
	TLSCAFile   string // CA certificate for verification
	TLSMutual   bool   // Require mutual TLS
}

// DefaultReplicationConfig returns sensible defaults.
func DefaultReplicationConfig() ReplicationConfig {
	return ReplicationConfig{
		Mode:           ModePrimary,
		ListenAddr:     ":9999",
		ReconnectDelay: 5 * time.Second,
		SyncTimeout:    30 * time.Second,
	}
}

// securityConfig converts ReplicationConfig to SecurityConfig.
func (c ReplicationConfig) securityConfig() SecurityConfig {
	return SecurityConfig{
		SecretKey:   c.SecretKey,
		TLSCertFile: c.TLSCertFile,
		TLSKeyFile:  c.TLSKeyFile,
		TLSCAFile:   c.TLSCAFile,
		TLSMutual:   c.TLSMutual,
	}
}

// Replicator handles state replication between nodes.
type Replicator struct {
	store  *SQLiteStore
	config ReplicationConfig
	logger *logging.Logger

	mu       sync.RWMutex
	replicas map[string]*replicaConn
	primary  *primaryConn

	ctx    context.Context
	cancel context.CancelFunc

	forceSnapshot bool // If true, next sync will request full snapshot
}

// replicaConn represents a connection to a replica.
type replicaConn struct {
	conn    net.Conn
	encoder *json.Encoder
	version uint64
}

// primaryConn represents a connection to the primary.
type primaryConn struct {
	conn    net.Conn
	decoder *json.Decoder
}

// NewReplicator creates a new replicator.
func NewReplicator(store *SQLiteStore, config ReplicationConfig, logger *logging.Logger) *Replicator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Replicator{
		store:    store,
		config:   config,
		logger:   logger,
		replicas: make(map[string]*replicaConn),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins replication based on mode.
func (r *Replicator) Start() error {
	switch r.config.Mode {
	case ModePrimary:
		return r.startPrimary()
	case ModeReplica:
		return r.startReplica()
	case ModeStandby:
		// Standby mode waits for explicit sync
		return nil
	default:
		return fmt.Errorf("unknown replication mode: %s", r.config.Mode)
	}
}

// Stop stops replication.
func (r *Replicator) Stop() {
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Close replica connections
	for addr, replica := range r.replicas {
		replica.conn.Close()
		delete(r.replicas, addr)
	}

	// Close primary connection
	if r.primary != nil {
		r.primary.conn.Close()
		r.primary = nil
	}
}

// SetMode dynamically changes the replication mode (e.g. for HA failover).
// It stops the current mode and starts the new one.
func (r *Replicator) SetMode(mode ReplicationMode) error {
	r.mu.Lock()
	if r.config.Mode == mode {
		r.mu.Unlock()
		return nil
	}
	r.logger.Info("Switching replication mode", "from", r.config.Mode, "to", mode)
	r.mu.Unlock() // Unlock to allow Stop() to acquire lock

	// Stop current operations
	r.Stop()

	r.mu.Lock()
	// Re-initialize context since Stop() cancels it
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.config.Mode = mode
	r.mu.Unlock()

	// Start new mode
	return r.Start()
}

// startPrimary starts the primary replication server.
func (r *Replicator) startPrimary() error {
	// Use secure listener (TLS if configured)
	listener, err := newSecureListener(r.config.ListenAddr, r.config.securityConfig())
	if err != nil {
		return fmt.Errorf("failed to start replication listener: %w", err)
	}

	r.logger.Info("Replication primary started", "addr", r.config.ListenAddr)

	// HA Recovery: Try to sync from peer if we are in HA mode and peer might be active
	if r.config.PeerAddr != "" {
		r.attemptReverseSync()
	}

	// Accept replica connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-r.ctx.Done():
					listener.Close()
					return
				default:
					r.logger.Warn("Failed to accept replica connection", "error", err)
					continue
				}
			}
			go r.handleReplica(conn)
		}
	}()

	// Subscribe to changes and broadcast to replicas
	go r.broadcastChanges()

	return nil
}

// attemptReverseSync tries to fetch the latest state from the peer before starting as primary.
// This handles the split-brain recovery case where the peer was promoted to primary while we were down.
func (r *Replicator) attemptReverseSync() {
	// Heuristic: Assume peer replication port matches our listen port
	// PeerAddr is usually the HA heartbeat address (e.g. 192.168.100.2:9002)
	// We need 192.168.100.2:9001 (if we listen on 9001)

	peerHost, _, err := net.SplitHostPort(r.config.PeerAddr)
	if err != nil {
		r.logger.Debug("Invalid peer address format, skipping reverse sync", "peer", r.config.PeerAddr)
		return
	}

	_, myPort, err := net.SplitHostPort(r.config.ListenAddr)
	if err != nil {
		r.logger.Debug("Invalid listen address format, skipping reverse sync", "listen", r.config.ListenAddr)
		return
	}

	target := net.JoinHostPort(peerHost, myPort)
	r.logger.Info("Attempting reverse sync from peer (HA recovery)", "target", target)

	// Short timeout - don't block startup too long
	// If peer is a replica (normal case), this will fail fast (connection refused)
	conn, err := dialSecure(target, r.config.securityConfig(), 2*time.Second)
	if err != nil {
		r.logger.Debug("Reverse sync failed (peer likely not primary)", "error", err)
		return
	}
	defer conn.Close()

	// Perform Sync
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// PSK Authentication
	if r.config.SecretKey != "" {
		conn.SetReadDeadline(clock.Now().Add(5 * time.Second))
		var challenge authChallenge
		if err := decoder.Decode(&challenge); err != nil {
			r.logger.Warn("Reverse sync: failed to read auth challenge", "error", err)
			return
		}
		mac := computeMAC(challenge.Nonce, []byte(r.config.SecretKey))
		if err := encoder.Encode(authResponse{MAC: mac}); err != nil {
			r.logger.Warn("Reverse sync: failed to send auth response", "error", err)
			return
		}
	}

	// Request Snapshot
	req := syncRequest{Version: 0}
	if err := encoder.Encode(req); err != nil {
		r.logger.Warn("Reverse sync: failed to send sync request", "error", err)
		return
	}

	conn.SetReadDeadline(clock.Now().Add(10 * time.Second))
	var resp syncResponse
	if err := decoder.Decode(&resp); err != nil {
		r.logger.Warn("Reverse sync: failed to read sync response", "error", err)
		return
	}

	if resp.Type == "snapshot" && resp.Snapshot != nil {
		if err := r.store.RestoreSnapshot(resp.Snapshot); err != nil {
			r.logger.Error("Reverse sync: failed to restore snapshot", "error", err)
		} else {
			r.logger.Info("Reverse sync successful! Restored state from peer.", "version", resp.Snapshot.Version)
		}
	} else {
		r.logger.Warn("Reverse sync: unexpected response type", "type", resp.Type)
	}
}

// handleReplica handles a new replica connection.
func (r *Replicator) handleReplica(conn net.Conn) {
	addr := conn.RemoteAddr().String()
	r.logger.Info("Replica connected", "addr", addr)

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// PSK Authentication handshake (if secret_key is configured)
	if r.config.SecretKey != "" {
		nonce, err := generateNonce()
		if err != nil {
			r.logger.Warn("Failed to generate nonce", "addr", addr, "error", err)
			conn.Close()
			return
		}

		// Send challenge
		if err := encoder.Encode(authChallenge{Nonce: nonce}); err != nil {
			r.logger.Warn("Failed to send auth challenge", "addr", addr, "error", err)
			conn.Close()
			return
		}

		// Receive response
		var resp authResponse
		if err := decoder.Decode(&resp); err != nil {
			r.logger.Warn("Failed to read auth response", "addr", addr, "error", err)
			conn.Close()
			return
		}

		// Verify HMAC
		if !verifyMAC(nonce, resp.MAC, []byte(r.config.SecretKey)) {
			r.logger.Warn("Authentication failed", "addr", addr)
			conn.Close()
			return
		}
		r.logger.Info("Replica authenticated", "addr", addr)
	}

	// Read sync request
	var req syncRequest
	if err := decoder.Decode(&req); err != nil {
		r.logger.Warn("Failed to read sync request", "addr", addr, "error", err)
		conn.Close()
		return
	}

	// Send snapshot or changes since version
	if req.Version == 0 {
		// Full sync
		snapshot, err := r.store.CreateSnapshot()
		if err != nil {
			r.logger.Warn("Failed to create snapshot", "error", err)
			conn.Close()
			return
		}

		resp := syncResponse{
			Type:     "snapshot",
			Snapshot: snapshot,
		}
		if err := encoder.Encode(resp); err != nil {
			r.logger.Warn("Failed to send snapshot", "error", err)
			conn.Close()
			return
		}
		r.logger.Info("Sent full snapshot to replica", "addr", addr, "version", snapshot.Version)
	} else {
		// Incremental sync
		changes, err := r.store.GetChangesSince(req.Version)
		if err != nil {
			r.logger.Warn("Failed to get changes", "error", err)
			conn.Close()
			return
		}

		resp := syncResponse{
			Type:    "changes",
			Changes: changes,
		}
		if err := encoder.Encode(resp); err != nil {
			r.logger.Warn("Failed to send changes", "error", err)
			conn.Close()
			return
		}
		r.logger.Info("Sent incremental changes to replica", "addr", addr, "count", len(changes))
	}

	// Register replica for ongoing updates
	r.mu.Lock()
	r.replicas[addr] = &replicaConn{
		conn:    conn,
		encoder: encoder,
		version: r.store.CurrentVersion(),
	}
	r.mu.Unlock()

	// Keep connection alive and handle disconnects
	go func() {
		buf := make([]byte, 1)
		for {
			conn.SetReadDeadline(clock.Now().Add(30 * time.Second))
			_, err := conn.Read(buf)
			if err != nil {
				r.mu.Lock()
				delete(r.replicas, addr)
				r.mu.Unlock()
				conn.Close()
				r.logger.Info("Replica disconnected", "addr", addr)
				return
			}
		}
	}()
}

// broadcastChanges subscribes to store changes and sends to all replicas.
func (r *Replicator) broadcastChanges() {
	changes := r.store.Subscribe(r.ctx)

	for change := range changes {
		r.mu.RLock()
		for addr, replica := range r.replicas {
			msg := replicationMessage{
				Type:   "change",
				Change: &change,
			}
			if err := replica.encoder.Encode(msg); err != nil {
				r.logger.Warn("Failed to send change to replica", "addr", addr, "error", err)
				// Will be cleaned up by the read goroutine
			}
		}
		r.mu.RUnlock()
	}
}

// startReplica connects to the primary and receives updates.
func (r *Replicator) startReplica() error {
	go r.replicaLoop()
	return nil
}

// replicaLoop maintains connection to primary.
func (r *Replicator) replicaLoop() {
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		if err := r.connectToPrimary(); err != nil {
			r.logger.Warn("Failed to connect to primary", "error", err)
			time.Sleep(r.config.ReconnectDelay)
			continue
		}

		// Receive updates until disconnected
		if err := r.receiveUpdates(); err != nil {
			if errors.Is(err, ErrDivergence) {
				r.logger.Error("Replication divergence detected! Forcing full snapshot sync on next connection.", "error", err)
				r.mu.Lock()
				r.forceSnapshot = true
				r.mu.Unlock()
			} else {
				r.logger.Warn("Lost connection to primary", "error", err)
			}

			r.mu.Lock()
			if r.primary != nil {
				r.primary.conn.Close()
				r.primary = nil
			}
			r.mu.Unlock()
			time.Sleep(r.config.ReconnectDelay)
		}
	}
}

// connectToPrimary establishes connection and performs initial sync.
func (r *Replicator) connectToPrimary() error {
	// Use secure dialer (TLS if configured)
	conn, err := dialSecure(r.config.PrimaryAddr, r.config.securityConfig(), r.config.SyncTimeout)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// PSK Authentication handshake (if secret_key is configured)
	if r.config.SecretKey != "" {
		// Receive challenge from server
		conn.SetReadDeadline(clock.Now().Add(5 * time.Second))
		var challenge authChallenge
		if err := decoder.Decode(&challenge); err != nil {
			conn.Close()
			return fmt.Errorf("failed to read auth challenge: %w", err)
		}

		// Compute and send HMAC response
		mac := computeMAC(challenge.Nonce, []byte(r.config.SecretKey))
		if err := encoder.Encode(authResponse{MAC: mac}); err != nil {
			conn.Close()
			return fmt.Errorf("failed to send auth response: %w", err)
		}
		r.logger.Debug("Authenticated with primary")
	}

	// Send sync request
	r.mu.RLock()
	requestVersion := r.store.CurrentVersion()
	if r.forceSnapshot {
		requestVersion = 0
	}
	r.mu.RUnlock()

	req := syncRequest{
		Version: requestVersion,
	}
	if err := encoder.Encode(req); err != nil {
		conn.Close()
		return err
	}

	// Receive response
	conn.SetReadDeadline(clock.Now().Add(30 * time.Second))
	var resp syncResponse
	if err := decoder.Decode(&resp); err != nil {
		conn.Close()
		return err
	}

	// Apply sync data
	switch resp.Type {
	case "snapshot":
		if err := r.store.RestoreSnapshot(resp.Snapshot); err != nil {
			conn.Close()
			return fmt.Errorf("failed to restore snapshot: %w", err)
		}
		r.logger.Info("Restored snapshot from primary", "version", resp.Snapshot.Version)

		// Reset forceSnapshot flag after successful restore
		r.mu.Lock()
		r.forceSnapshot = false
		r.mu.Unlock()

	case "changes":
		for _, change := range resp.Changes {
			if err := r.applyChange(change); err != nil {
				r.logger.Warn("Failed to apply change", "error", err)
			}
		}
		r.logger.Info("Applied incremental changes", "count", len(resp.Changes))
	}

	r.mu.Lock()
	r.primary = &primaryConn{
		conn:    conn,
		decoder: decoder,
	}
	r.mu.Unlock()

	r.logger.Info("Connected to primary", "addr", r.config.PrimaryAddr)
	return nil
}

// receiveUpdates receives and applies changes from primary.
func (r *Replicator) receiveUpdates() error {
	r.mu.RLock()
	primary := r.primary
	r.mu.RUnlock()

	if primary == nil {
		return fmt.Errorf("not connected to primary")
	}

	for {
		var msg replicationMessage
		if err := primary.decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return fmt.Errorf("primary closed connection")
			}
			return err
		}

		switch msg.Type {
		case "change":
			if msg.Change != nil {
				if err := r.applyChange(*msg.Change); err != nil {
					r.logger.Warn("Failed to apply change", "error", err)
				}
			}
		}
	}
}

// applyChange applies a replicated change to the local store.
func (r *Replicator) applyChange(change Change) error {
	return r.store.ApplyReplicatedChange(change)
}

// SyncFromPeer performs a one-time sync from another node.
// Used for upgrades and initial HA setup.
func (r *Replicator) SyncFromPeer(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, r.config.SyncTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Request full sync
	req := syncRequest{Version: 0}
	if err := encoder.Encode(req); err != nil {
		return err
	}

	var resp syncResponse
	if err := decoder.Decode(&resp); err != nil {
		return err
	}

	if resp.Type != "snapshot" {
		return fmt.Errorf("expected snapshot, got %s", resp.Type)
	}

	return r.store.RestoreSnapshot(resp.Snapshot)
}

// ExportForUpgrade creates a snapshot for upgrade handoff.
func (r *Replicator) ExportForUpgrade() (*Snapshot, error) {
	return r.store.CreateSnapshot()
}

// ImportFromUpgrade restores state from an upgrade handoff.
func (r *Replicator) ImportFromUpgrade(snapshot *Snapshot) error {
	return r.store.RestoreSnapshot(snapshot)
}

// Protocol messages

type syncRequest struct {
	Version uint64 `json:"version"`
}

type syncResponse struct {
	Type     string    `json:"type"` // "snapshot" or "changes"
	Snapshot *Snapshot `json:"snapshot,omitempty"`
	Changes  []Change  `json:"changes,omitempty"`
}

type replicationMessage struct {
	Type   string  `json:"type"` // "change"
	Change *Change `json:"change,omitempty"`
}

// ReplicatorStatus contains the operational status of the replicator.
type ReplicatorStatus struct {
	Mode         string `json:"mode"`
	Connected    bool   `json:"connected"`
	PeerAddress  string `json:"peer_address"`
	SyncState    string `json:"sync_state"`
	Version      uint64 `json:"version"`
	ReplicaCount int    `json:"replica_count"`
}

// Status returns the current operational status.
func (r *Replicator) Status() ReplicatorStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := ReplicatorStatus{
		Mode:        string(r.config.Mode),
		Version:     r.store.CurrentVersion(),
		PeerAddress: r.config.ListenAddr,
	}

	if r.config.Mode == ModeReplica {
		status.PeerAddress = r.config.PrimaryAddr
		if r.primary != nil {
			status.Connected = true
			status.SyncState = "synced"
		} else {
			status.SyncState = "connecting"
		}
	} else if r.config.Mode == ModePrimary {
		status.ReplicaCount = len(r.replicas)
		status.Connected = len(r.replicas) > 0
		status.SyncState = "serving"
	}

	return status
}

// CurrentVersion returns the current state version.
func (r *Replicator) CurrentVersion() uint64 {
	return r.store.CurrentVersion()
}
