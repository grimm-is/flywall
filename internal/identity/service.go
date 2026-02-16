// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package identity

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/firewall"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/state"
)

const (
	BucketIdentities = "identities"
	BucketGroups     = "groups"
)

type Service struct {
	mu sync.RWMutex

	store state.Store

	identities map[string]*DeviceIdentity // ID -> Identity
	groups     map[string]*DeviceGroup    // ID -> Group

	// Index
	macToIdentity map[string]string // MAC -> IdentityID

	// Firewall Integration
	fwMgr    *firewall.Manager
	ipsetSvc *firewall.IPSetService
}

func NewService(store state.Store) *Service {
	s := &Service{
		store:         store,
		identities:    make(map[string]*DeviceIdentity),
		groups:        make(map[string]*DeviceGroup),
		macToIdentity: make(map[string]string),
	}

	// Create buckets if they don't exist
	if err := store.CreateBucket(BucketIdentities); err != nil && err != state.ErrBucketExists {
		logging.Error("Failed to create identities bucket", "error", err)
	}
	if err := store.CreateBucket(BucketGroups); err != nil && err != state.ErrBucketExists {
		logging.Error("Failed to create groups bucket", "error", err)
	}

	// Load initial state
	if err := s.loadState(); err != nil {
		logging.Error("Failed to load identity state", "error", err)
	}

	return s
}

// SetFirewallDependencies injects firewall services for group enforcement
func (s *Service) SetFirewallDependencies(fwMgr *firewall.Manager, ipsetSvc *firewall.IPSetService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fwMgr = fwMgr
	s.ipsetSvc = ipsetSvc
}

// loadState reads all identities and groups from storage
func (s *Service) loadState() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load Groups
	groups, err := s.store.List(BucketGroups)
	if err != nil {
		return fmt.Errorf("list groups: %w", err)
	}
	for id, data := range groups {
		var g DeviceGroup
		// We stored JSON, but List returns []byte. Need to unmarshal?
		// state.Store doesn't enforce format, but GetJSON/SetJSON wrap json.Unmarshal.
		// Use standard json.Unmarshal here.
		// Wait, state.Store has List that returns []byte values.
		// We should probably rely on valid JSON.
		// But let's assume we used SetJSON.
		// Actually, I can use a helper or just json.Unmarshal.
		if err := json.Unmarshal(data, &g); err != nil {
			logging.Warn("Failed to unmarshal group", "id", id, "error", err)
			continue
		}
		s.groups[id] = &g
	}

	// Load Identities
	idents, err := s.store.List(BucketIdentities)
	if err != nil {
		return fmt.Errorf("list identities: %w", err)
	}
	for id, data := range idents {
		var identity DeviceIdentity
		if err := json.Unmarshal(data, &identity); err != nil {
			logging.Warn("Failed to unmarshal identity", "id", id, "error", err)
			continue
		}
		s.identities[id] = &identity
		// Build index
		for _, mac := range identity.MACs {
			s.macToIdentity[mac] = identity.ID
		}
	}

	logging.Info("Loaded identity state", "identities", len(s.identities), "groups", len(s.groups))
	return nil
}

// LoadConfig loads groups from config (if persisted there)
func (s *Service) LoadConfig(cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// TODO: Load from config or persistence
	return nil
}

// IdentifyDevice returns the identity for a MAC, creating a transient one if unknown
func (s *Service) IdentifyDevice(mac string) *DeviceIdentity {
	s.mu.RLock()
	id, ok := s.macToIdentity[mac]
	s.mu.RUnlock()

	if ok {
		s.mu.RLock()
		identity := s.identities[id]
		s.mu.RUnlock()
		if identity != nil {
			return identity.Clone()
		}
	}

	// Create new persistent identity
	newID := &DeviceIdentity{
		ID:        uuid.New().String(),
		MACs:      []string{mac},
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}

	s.mu.Lock()
	s.identities[newID.ID] = newID
	s.macToIdentity[mac] = newID.ID
	s.mu.Unlock()

	if err := s.saveIdentity(newID); err != nil {
		logging.Error("Failed to persist new identity", "error", err)
	}

	return newID.Clone()
}

// GetIdentity returns a specific identity by ID
func (s *Service) GetIdentity(id string) *DeviceIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id, ok := s.identities[id]; ok {
		return id.Clone()
	}
	return nil
}

// UpdateIdentity updates an existing identity
func (s *Service) UpdateIdentity(update *DeviceIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.identities[update.ID]
	if !ok {
		return fmt.Errorf("identity not found: %s", update.ID)
	}

	// Update fields (preserve immutable ones like ID, FirstSeen if needed, but assuming update has them)
	// We trust the caller to provide complete object or we should merge?
	// For now, replace.
	// Check if MACs changed? LinkMAC/UnlinkMAC should handle MAC changes to keep index in sync.
	// If caller manually changed MACs, index might break.
	// Let's enforce MACs are read-only via UpdateIdentity or handle index update.
	// Better to ignore MACs in UpdateIdentity and force use of Link/Unlink.

	update.MACs = current.MACs // Restore MACs from source of truth
	s.identities[update.ID] = update

	// Sync groups if changed
	if current.GroupID != "" {
		s.syncGroupToFirewallLocked(current.GroupID)
	}
	if update.GroupID != "" && update.GroupID != current.GroupID {
		s.syncGroupToFirewallLocked(update.GroupID)
	}

	return s.saveIdentity(update)
}

// LinkMAC moves a MAC to a target identity
func (s *Service) LinkMAC(mac, targetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target, ok := s.identities[targetID]
	if !ok {
		return fmt.Errorf("target identity not found: %s", targetID)
	}

	currentID, hasIdentity := s.macToIdentity[mac]
	if hasIdentity && currentID == targetID {
		return nil // Already linked
	}

	// Remove from old identity
	if hasIdentity {
		if old, ok := s.identities[currentID]; ok {
			newMACs := make([]string, 0, len(old.MACs)-1)
			for _, m := range old.MACs {
				if m != mac {
					newMACs = append(newMACs, m)
				}
			}
			old.MACs = newMACs
			s.saveIdentity(old)
		}
	}

	// Add to new identity
	target.MACs = append(target.MACs, mac)
	s.macToIdentity[mac] = targetID
	s.saveIdentity(target)

	// Sync firewall
	if target.GroupID != "" {
		s.syncGroupToFirewallLocked(target.GroupID)
	}
	// Sync old identity's group if it existed
	if hasIdentity {
		if old, ok := s.identities[currentID]; ok {
			if old.GroupID != "" {
				s.syncGroupToFirewallLocked(old.GroupID)
			}
		}
	}

	return nil
}

// UnlinkMAC removes a MAC from its identity (effectively creating a new one later)
func (s *Service) UnlinkMAC(mac string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentID, ok := s.macToIdentity[mac]
	if !ok {
		return nil // Not linked
	}

	identity, ok := s.identities[currentID]
	if !ok {
		return nil
	}

	// Remove MAC
	newMACs := make([]string, 0, len(identity.MACs)-1)
	for _, m := range identity.MACs {
		if m != mac {
			newMACs = append(newMACs, m)
		}
	}
	identity.MACs = newMACs
	delete(s.macToIdentity, mac)

	if identity.GroupID != "" {
		s.syncGroupToFirewallLocked(identity.GroupID)
	}

	return s.saveIdentity(identity)
}

// saveIdentity persists an identity to the store
func (s *Service) saveIdentity(identity *DeviceIdentity) error {
	return s.store.SetJSON(BucketIdentities, identity.ID, identity)
}

// GetGroups returns all groups
func (s *Service) GetGroups() []DeviceGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groups := make([]DeviceGroup, 0, len(s.groups))
	for _, g := range s.groups {
		groups = append(groups, *g)
	}
	return groups
}

// UpdateGroup creates or updates a group
func (s *Service) UpdateGroup(g DeviceGroup) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if g.ID == "" {
		g.ID = uuid.New().String()
	}

	s.groups[g.ID] = &g

	// Persist
	if err := s.store.SetJSON(BucketGroups, g.ID, g); err != nil {
		return fmt.Errorf("persist group: %w", err)
	}

	s.syncGroupToFirewallLocked(g.ID)
	return nil
}

// DeleteGroup deletes a group
func (s *Service) DeleteGroup(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.groups, id)
	if err := s.store.Delete(BucketGroups, id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	s.deleteGroupFromFirewallLocked(id)
	return nil
}
