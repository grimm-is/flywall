// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package vpn

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWireGuardConfig_MarshalJSON_MasksPrivateKey(t *testing.T) {
	config := WireGuardConfig{
		Enabled:    true,
		PrivateKey: "private-key-12345",
		Interface:  "wg0",
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, "private-key-12345") {
		t.Error("JSON output should NOT contain actual private key")
	}
	if !strings.Contains(jsonStr, "(hidden)") {
		t.Error("JSON output SHOULD contain masked private key")
	}
}
