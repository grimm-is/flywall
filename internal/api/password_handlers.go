// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"

	"grimm.is/flywall/internal/auth"
)

// handleChangePassword allows an authenticated user to change their own password
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	// 0. Get user from context (populated by auth middleware)
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 1. Verify current password
	// We re-authenticate to verify the current password is correct
	// This prevents session hijacking from allowing password changes
	_, err := s.authStore.Authenticate(user.Username, req.CurrentPassword)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid current password")
		return
	}

	// 2. Validate new password strength
	policy := auth.DefaultPasswordPolicy()
	if err := auth.ValidatePassword(req.NewPassword, policy, user.Username); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 3. Update password
	if err := s.authStore.UpdatePassword(user.Username, req.NewPassword); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update password: "+err.Error())
		return
	}

	// Audit log
	s.audit(r, "auth.password_change", "user changed own password", nil)

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleAdminResetPassword allows an admin to force-reset another user's password
func (s *Server) handleAdminResetPassword(w http.ResponseWriter, r *http.Request) {
	// Authz check: must be admin (handled by require middleware, but double check doesn't hurt)
	// The middleware s.require(storage.PermAdminSystem) should be used on the route.

	targetUsername := r.PathValue("username")
	if targetUsername == "" {
		WriteError(w, http.StatusBadRequest, "username required")
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 1. Validate new password strength
	policy := auth.DefaultPasswordPolicy()
	if err := auth.ValidatePassword(req.NewPassword, policy, targetUsername); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 2. Check if user exists
	if _, err := s.authStore.GetUser(targetUsername); err != nil {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	// 3. Update password
	if err := s.authStore.UpdatePassword(targetUsername, req.NewPassword); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to reset password: "+err.Error())
		return
	}

	// Audit log
	s.audit(r, "auth.password_reset", "admin reset password for user "+targetUsername, map[string]interface{}{
		"target_user": targetUsername,
	})

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}
