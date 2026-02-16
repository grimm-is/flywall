// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// Clone returns a deep copy of the configuration.
// Uses gob encoding to avoid issues with JSON field name transformations.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	dec := gob.NewDecoder(&buf)

	if err := enc.Encode(c); err != nil {
		fmt.Printf("CLONE ERROR: Failed to encode config: %v\n", err)
		return nil
	}

	var clone Config
	if err := dec.Decode(&clone); err != nil {
		fmt.Printf("CLONE ERROR: Failed to decode config: %v\n", err)
		return nil
	}

	return &clone
}
