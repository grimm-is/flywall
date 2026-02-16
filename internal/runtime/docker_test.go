// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package runtime

import (
	"strings"
	"testing"
)

func TestParseContainers(t *testing.T) {
	jsonResp := `[
		{
			"Id": "8dfafdbc3a40",
			"Names": ["/boring_feynman"],
			"Image": "ubuntu:latest",
			"State": "running",
			"Status": "Up 2 hours",
			"NetworkSettings": {
				"Networks": {
					"bridge": {
						"IPAddress": "172.17.0.2",
						"GlobalIPv6Address": "",
						"Gateway": "172.17.0.1",
						"MacAddress": "02:42:ac:11:00:02"
					}
				}
			},
			"Labels": {
				"com.docker.compose.project": "flywall"
			}
		}
	]`

	reader := strings.NewReader(jsonResp)
	containers, err := parseContainers(reader)
	if err != nil {
		t.Fatalf("Failed to parse containers: %v", err)
	}

	if len(containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(containers))
	}

	c := containers[0]
	if c.ID != "8dfafdbc3a40" {
		t.Errorf("Expected ID 8dfafdbc3a40, got %s", c.ID)
	}
	if len(c.Names) == 0 || c.Names[0] != "/boring_feynman" {
		t.Errorf("Expected name /boring_feynman, got %v", c.Names)
	}
	if c.NetworkSettings.Networks["bridge"].IPAddress != "172.17.0.2" {
		t.Errorf("Expected IP 172.17.0.2, got %s", c.NetworkSettings.Networks["bridge"].IPAddress)
	}
}
