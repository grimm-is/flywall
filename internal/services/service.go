// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package services

import (
	"context"
	"grimm.is/flywall/internal/config"
)

// ServiceStatus represents the current state of a service.
type ServiceStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// Service defines the standard lifecycle methods for all services.
type Service interface {
	// Name returns the unique name of the service.
	Name() string

	// Reload applies the given configuration to the service.
	// It returns true if the service was restarted, and an error if one occurred.
	Reload(cfg *config.Config) (bool, error)

	// Start starts the service.
	Start(ctx context.Context) error

	// Stop stops the service.
	Stop(ctx context.Context) error

	// Status returns the current status of the service.
	Status() ServiceStatus
}
