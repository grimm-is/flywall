// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"grimm.is/flywall/internal/auth"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// MockStore wraps DevStore to allow overrides
type MockStore struct {
	*auth.DevStore
	updatePasswordFunc func(username, newPassword string) error
	authenticateFunc   func(username, password string) (*auth.Session, error)
	getUserFunc        func(username string) (*auth.User, error)
}

func (m *MockStore) UpdatePassword(username, newPassword string) error {
	if m.updatePasswordFunc != nil {
		return m.updatePasswordFunc(username, newPassword)
	}
	return m.DevStore.UpdatePassword(username, newPassword)
}

func (m *MockStore) Authenticate(username, password string) (*auth.Session, error) {
	if m.authenticateFunc != nil {
		return m.authenticateFunc(username, password)
	}
	return m.DevStore.Authenticate(username, password)
}

func (m *MockStore) GetUser(username string) (*auth.User, error) {
	if m.getUserFunc != nil {
		return m.getUserFunc(username)
	}
	return m.DevStore.GetUser(username)
}

func TestHandleChangePassword(t *testing.T) {
	logger := logging.New(logging.DefaultConfig())
	mockAuth := &MockStore{DevStore: auth.NewDevStore()}

	// Create a user context simulating authenticated user
	user := &auth.User{Username: "testuser", Role: auth.RoleAdmin}

	server := &Server{
		Config:    &config.Config{},
		logger:    logger,
		authStore: mockAuth,
	}

	tests := []struct {
		name           string
		body           map[string]string
		mockAuth       func()
		expectedStatus int
	}{
		{
			name: "Success",
			body: map[string]string{
				"current_password": "currentPassword123",
				"new_password":     "newSecurePassword123!",
			},
			mockAuth: func() {
				mockAuth.authenticateFunc = func(u, p string) (*auth.Session, error) {
					if p == "currentPassword123" {
						return &auth.Session{}, nil
					}
					return nil, errors.New("invalid credentials")
				}
				mockAuth.updatePasswordFunc = func(u, p string) error {
					return nil
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Wrong Current Password",
			body: map[string]string{
				"current_password": "wrongPassword",
				"new_password":     "newSecurePassword123!",
			},
			mockAuth: func() {
				mockAuth.authenticateFunc = func(u, p string) (*auth.Session, error) {
					return nil, errors.New("invalid credentials")
				}
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Weak New Password",
			body: map[string]string{
				"current_password": "currentPassword123",
				"new_password":     "short",
			},
			mockAuth: func() {
				mockAuth.authenticateFunc = func(u, p string) (*auth.Session, error) {
					return &auth.Session{}, nil
				}
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockAuth()

			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewReader(bodyBytes))

			// Inject user into context (simulating middleware)
			ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			server.handleChangePassword(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandleAdminResetPassword(t *testing.T) {
	logger := logging.New(logging.DefaultConfig())
	mockAuth := &MockStore{DevStore: auth.NewDevStore()}

	server := &Server{
		Config:    &config.Config{},
		logger:    logger,
		authStore: mockAuth,
	}

	tests := []struct {
		name           string
		targetUser     string
		body           map[string]string
		mockSetup      func()
		expectedStatus int
	}{
		{
			name:       "Success",
			targetUser: "victim",
			body: map[string]string{
				"new_password": "newSecurePassword123!",
			},
			mockSetup: func() {
				mockAuth.getUserFunc = func(u string) (*auth.User, error) {
					return &auth.User{Username: "victim"}, nil
				}
				mockAuth.updatePasswordFunc = func(u, p string) error {
					return nil
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "User Not Found",
			targetUser: "ghost",
			body: map[string]string{
				"new_password": "newSecurePassword123!",
			},
			mockSetup: func() {
				mockAuth.getUserFunc = func(u string) (*auth.User, error) {
					return nil, errors.New("not found")
				}
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "Weak Password",
			targetUser: "victim",
			body: map[string]string{
				"new_password": "123",
			},
			mockSetup: func() {
				// No need to mock store as verification fails before call
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("PUT", "/api/users/"+tt.targetUser+"/password", bytes.NewReader(bodyBytes))
			req.SetPathValue("username", tt.targetUser)

			w := httptest.NewRecorder()
			server.handleAdminResetPassword(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
