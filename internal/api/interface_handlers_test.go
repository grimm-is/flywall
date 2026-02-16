// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

func TestHandleInterfaces_NoClient(t *testing.T) {
	logger := logging.New(logging.DefaultConfig())

	cfg := &config.Config{
		Interfaces: []config.Interface{
			{Name: "eth0", Zone: "WAN", IPv4: []string{"dhcp"}},
			{Name: "eth1", Zone: "LAN", IPv4: []string{"10.0.0.1/24"}},
		},
	}

	server := &Server{
		Config: cfg,
		logger: logger,
		// No client - should return 503
	}

	req, _ := http.NewRequest("GET", "/api/interfaces", nil)
	rr := httptest.NewRecorder()

	server.handleInterfaces(rr, req)

	// Without RPC client, handleInterfaces returns ServiceUnavailable
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("expected ServiceUnavailable without client, got %v", status)
	}
}
