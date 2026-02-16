// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package identity

import (
	"time"
)

// Schedule represents a weekly time schedule for access control
type Schedule struct {
	Enabled  bool        `json:"enabled"`
	Timezone string      `json:"timezone"` // e.g. "America/New_York"
	Blocks   []TimeBlock `json:"blocks"`
}

// TimeBlock represents a specific time range on specific days
type TimeBlock struct {
	Days      []string `json:"days"`       // "Mon", "Tue", etc.
	StartTime string   `json:"start_time"` // "HH:MM" 24h
	EndTime   string   `json:"end_time"`   // "HH:MM" 24h
}

// DeviceIdentity represents a tracked device
type DeviceIdentity struct {
	ID        string    `json:"id"`
	MACs      []string  `json:"macs"` // Primary MAC is first
	Alias     string    `json:"alias"`
	Owner     string    `json:"owner"`
	GroupID   string    `json:"group_id"`
	Tags      []string  `json:"tags"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// DeviceGroup represents a logical group of devices
type DeviceGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	Icon        string    `json:"icon"`
	Schedule    *Schedule `json:"schedule,omitempty"`
	TargetPolicy string   `json:"target_policy,omitempty"` // Policy to apply blocks to (e.g. "lan_wan")
}

// Clone returns a deep copy of DeviceIdentity
func (d *DeviceIdentity) Clone() *DeviceIdentity {
	macs := make([]string, len(d.MACs))
	copy(macs, d.MACs)

	tags := make([]string, len(d.Tags))
	copy(tags, d.Tags)

	return &DeviceIdentity{
		ID:        d.ID,
		MACs:      macs,
		Alias:     d.Alias,
		Owner:     d.Owner,
		GroupID:   d.GroupID,
		Tags:      tags,
		FirstSeen: d.FirstSeen,
		LastSeen:  d.LastSeen,
	}
}
