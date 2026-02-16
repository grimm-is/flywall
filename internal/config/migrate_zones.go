// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// migrate_zones.go canonicalizes deprecated zone fields:
//   - Interface.Zone -> Zone.Match
//   - Zone.Interfaces -> Zone.Match

// findOrCreateZoneForMigration returns a pointer to the zone with the given name.
// If it doesn't exist, it creates it.
func findOrCreateZoneForMigration(c *Config, name string) *Zone {
	for i := range c.Zones {
		if c.Zones[i].Name == name {
			return &c.Zones[i]
		}
	}

	newZone := Zone{
		Name:    name,
		Matches: []RuleMatch{},
	}
	c.Zones = append(c.Zones, newZone)
	return &c.Zones[len(c.Zones)-1]
}

func canonicalizeZones(c *Config) error {
	// 1. Migrate Interface.Zone -> Zone.Match
	for i := range c.Interfaces {
		iface := &c.Interfaces[i]
		if iface.Zone != "" {
			zone := findOrCreateZoneForMigration(c, iface.Zone)

			if zone.Matches == nil {
				zone.Matches = []RuleMatch{}
			}

			alreadyMatched := false
			for _, m := range zone.Matches {
				if m.Interface == iface.Name {
					alreadyMatched = true
					break
				}
			}

			if !alreadyMatched {
				zone.Matches = append(zone.Matches, RuleMatch{
					Interface: iface.Name,
				})
			}

			iface.Zone = ""
		}
	}

	// 2. Migrate Zone.Interface (singular) -> Zone.Match
	for i := range c.Zones {
		zone := &c.Zones[i]

		if zone.Interface != "" {
			if zone.Matches == nil {
				zone.Matches = []RuleMatch{}
			}

			// Check for dupes
			alreadyMatched := false
			for _, m := range zone.Matches {
				if m.Interface == zone.Interface {
					alreadyMatched = true
					break
				}
			}

			if !alreadyMatched {
				zone.Matches = append(zone.Matches, RuleMatch{
					Interface: zone.Interface,
				})
			}

			zone.Interface = ""
		}
	}

	return nil
}

func init() {
	RegisterPostLoadMigration(PostLoadMigration{
		Name:        "zone_canonicalize",
		Description: "Migrate deprecated zone fields to canonical Match blocks",
		Migrate:     canonicalizeZones,
	})
}
