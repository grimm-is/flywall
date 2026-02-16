// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package auth

import (
	"testing"
)

// TestValidatePassword tests the main password validation function
func TestValidatePassword(t *testing.T) {
	policy := DefaultPasswordPolicy()

	tests := []struct {
		name      string
		password  string
		username  string
		wantError bool
		errorMsg  string
	}{
		// Happy paths
		{
			name:      "strong password",
			password:  "MyS3cur3P@ssw0rd!",
			username:  "",
			wantError: false,
		},
		{
			name: "long lowercase password",
			// 25 chars * log2(26) = 25 * 4.7 = 117 bits
			password:  "verylonglowercasepassword",
			username:  "",
			wantError: false,
		},
		{
			name: "password with diverse characters",
			// 15 chars * log2(95) = 15 * 6.5 = 98 bits
			password:  "Abc123!@#XyzPqr",
			username:  "testuser",
			wantError: false,
		},

		// Sad paths
		{
			name:      "empty password",
			password:  "",
			username:  "",
			wantError: true,
			errorMsg:  "password cannot be empty",
		},
		{
			name: "short weak password",
			// 4 * log2(26) = 18.8 bits -> Weak (<40)
			password:  "weak",
			username:  "",
			wantError: true,
			errorMsg:  "is too weak",
		},
		{
			name:      "password literal",
			password:  "password",
			username:  "",
			wantError: true,
			errorMsg:  "is too weak",
		},
		{
			name:      "contains username",
			password:  "admin123!@#",
			username:  "admin",
			wantError: true,
			errorMsg:  "is too weak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.username != "" {
				err = ValidatePassword(tt.password, policy, tt.username)
			} else {
				err = ValidatePassword(tt.password, policy)
			}

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidatePassword() expected error but got nil")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidatePassword() error = %v, want error containing %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePassword() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestCalculateStrength tests the entropy calculation with penalties
func TestCalculateStrength(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		username       string
		wantMinEntropy float64
		wantMaxEntropy float64
		wantScore      int // 0-4
	}{
		{
			name: "mixed case 12 chars",
			// 12 * log2(52) = 12 * 5.7 = 68.4 bits
			// No penalties.
			password:       "GoLangIsCool",
			username:       "",
			wantMinEntropy: 68.0,
			wantMaxEntropy: 69.0,
			wantScore:      2, // < 70 is Medium (2)
		},
		{
			name: "repetition penalty",
			// "aaaaa" (5 chars). log2(26)=4.7. Base = 23.5 bits.
			// Penalty: -15. Result: 8.5.
			password:       "aaaaa",
			username:       "",
			wantMinEntropy: 8.0,
			wantMaxEntropy: 9.0,
			wantScore:      1, // Weak
		},
		{
			name: "sequence penalty",
			// "abcde" (5 chars). Base = 23.5 bits.
			// Penalty: -15. Result: 8.5.
			password:       "abcde",
			username:       "",
			wantMinEntropy: 8.0,
			wantMaxEntropy: 9.0,
			wantScore:      1,
		},
		{
			name:           "password literal",
			password:       "password",
			wantMinEntropy: 0.0,
			wantMaxEntropy: 0.1,
			wantScore:      1,
		},
		{
			name:           "username deduction",
			password:       "admin123",
			username:       "admin",
			wantMinEntropy: 0.0,
			wantMaxEntropy: 0.1,
			wantScore:      1,
		},
		{
			name: "long strong password",
			// 20 chars, mixed alphanumeric -> pool 62
			// 20 * 5.95 = 119 bits.
			// No patterns.
			password:       "CorrectBatteryHorseStaple123",
			username:       "",
			wantMinEntropy: 100.0,
			wantMaxEntropy: 200.0,
			wantScore:      4, // Strong
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strength := CalculateStrength(tt.password, tt.username)

			if strength.Entropy < tt.wantMinEntropy || strength.Entropy > tt.wantMaxEntropy {
				t.Errorf("CalculateStrength() entropy = %v, want between %v and %v",
					strength.Entropy, tt.wantMinEntropy, tt.wantMaxEntropy)
			}

			if strength.Score != tt.wantScore {
				t.Errorf("CalculateStrength() score = %v, want %v", strength.Score, tt.wantScore)
			}
		})
	}
}

// TestHasPatterns tests the internal pattern checking helpers indirectly via CalculateStrength
// or directly if we export them. Since they are private, we rely on CalculateStrength tests above.
// But we can check coverage via side effects (entropy drops).

// Helper functions for tests

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
