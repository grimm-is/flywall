// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux
// +build !linux

package ha

import (
	"fmt"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// Role represents the current HA role of this node.
type Role string

const (
	RolePrimary    Role = "primary"
	RoleBackup     Role = "backup"
	RoleTakingOver Role = "taking_over"
	RoleFailed     Role = "failed"
)

// Default configuration values.
const (
	DefaultHeartbeatInterval = 1
	DefaultFailureThreshold  = 3
	DefaultHeartbeatPort     = 9002
	DefaultPriority          = 100
	DefaultFailbackDelay     = 60
)

// PeerState tracks the state of the peer node.
type PeerState struct {
	Alive            bool
	Role             Role
	Priority         int
	StateVersion     uint64
	MissedHeartbeats int
}

// LinkManager defines the interface for link layer operations.
type LinkManager interface {
	SetHardwareAddr(name string, mac []byte) error
	GetHardwareAddr(name string) ([]byte, error)
}

// Service is a stub for non-Linux platforms.
type Service struct{}

// NewService returns an error on non-Linux platforms.
func NewService(cfg *config.ReplicationConfig, nodeID string, linkMgr LinkManager, logger *logging.Logger) (*Service, error) {
	return nil, fmt.Errorf("HA service is only supported on Linux")
}

// OnBecomePrimary is a no-op on non-Linux.
func (s *Service) OnBecomePrimary(fn func() error) {}

// OnBecomeBackup is a no-op on non-Linux.
func (s *Service) OnBecomeBackup(fn func() error) {}

// Start is a no-op on non-Linux.
func (s *Service) Start() error {
	return fmt.Errorf("HA service is only supported on Linux")
}

// Stop is a no-op on non-Linux.
func (s *Service) Stop() {}

// GetRole returns RoleFailed on non-Linux.
func (s *Service) GetRole() Role {
	return RoleFailed
}

// GetPeerState returns an empty state on non-Linux.
func (s *Service) GetPeerState() PeerState {
	return PeerState{}
}

// TriggerFailover returns an error on non-Linux.
func (s *Service) TriggerFailover() error {
	return fmt.Errorf("HA service is only supported on Linux")
}

// DHCPReclaimer defines the interface for DHCP lease reclaim operations.
type DHCPReclaimer interface {
	ReclaimLease(ifaceName string) (interface{}, error)
}

// SetDHCPReclaimer is a no-op on non-Linux.
func (s *Service) SetDHCPReclaimer(mgr DHCPReclaimer) {}

// Replicator defines the interface for getting state version.
type Replicator interface {
	CurrentVersion() uint64
}

// SetReplicator is a no-op on non-Linux.
func (s *Service) SetReplicator(r Replicator) {}
