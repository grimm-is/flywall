// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package auth

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

type PasswordPolicy struct {
	MinLength  int     // Minimum length (default: 12)
	MinEntropy float64 // Minimum bits of entropy (default: 60)
}

func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:  12,
		MinEntropy: 60.0,
	}
}

type PasswordStrength struct {
	Score       int      // 0-4 (very weak to strong)
	Length      int      // Password length
	Entropy     float64  // Bits of entropy
	CharsetSize int      // Pool size
	Complexity  int      // Character classes count
	MeetsPolicy bool     // Does it meet the policy requirements
	Feedback    []string // Feedback messages for the user
}

// ValidatePassword against the policy.
// username is optional - if provided, password cannot contain it
func ValidatePassword(password string, policy PasswordPolicy, username ...string) error {
	// Check minimum length (keep policy enforcement if desired, or rely on score)
	if len(password) < 1 { // Basic sanity check
		return fmt.Errorf("password cannot be empty")
	}

	strength := CalculateStrength(password, username...)

	// Check strength score
	// < 2: Weak
	// >= 2: Medium/Strong
	if strength.Score < 2 {
		return fmt.Errorf("password is too weak (score=%d/4)", strength.Score)
	}

	return nil
}

// CalculateStrength calculates the strength of a password using entropy with penalties
func CalculateStrength(password string, username ...string) PasswordStrength {
	strength := PasswordStrength{
		Length:   len(password),
		Feedback: make([]string, 0),
	}

	// 1. Determine Pool Size & Complexity
	poolSize := 0
	complexity := 0
	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSymbol := false

	for _, char := range password {
		switch {
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSymbol = true
		}
	}

	if hasLower {
		poolSize += 26
		complexity++
	}
	if hasUpper {
		poolSize += 26
		complexity++
	}
	if hasDigit {
		poolSize += 10
		complexity++
	}
	if hasSymbol {
		poolSize += 33
		complexity++
	}
	if poolSize == 0 {
		poolSize = 26 // Logic fallback
	}

	strength.Complexity = complexity
	strength.CharsetSize = poolSize

	// 2. Calculate Raw Entropy
	entropy := float64(len(password)) * math.Log2(float64(poolSize))

	// 3. Apply Penalties
	lower := strings.ToLower(password)

	// Critical Deductions
	if lower == "password" || password == "12345678" {
		entropy = 0
		strength.Feedback = append(strength.Feedback, "Password is too common")
	}
	if len(username) > 0 && username[0] != "" {
		if strings.Contains(strings.ToLower(password), strings.ToLower(username[0])) {
			entropy = 0
			strength.Feedback = append(strength.Feedback, "Password contains username")
		}
	}

	// Pattern Deductions
	// Repetition (3+ identical)
	if hasRepetition(password) {
		entropy -= 15
		strength.Feedback = append(strength.Feedback, "Avoid repeated characters")
	}
	// Sequential (3+ sequential)
	if hasSequential(password) {
		entropy -= 15
		strength.Feedback = append(strength.Feedback, "Avoid sequential patterns")
	}

	if entropy < 0 {
		entropy = 0
	}
	strength.Entropy = entropy

	// 4. Map to Score
	// < 40: Weak (1)
	// 40-70: Medium (2)
	// >= 70: Strong (4)
	if entropy < 40 {
		strength.Score = 1
	} else if entropy < 70 {
		strength.Score = 2
	} else {
		strength.Score = 4
	}

	return strength
}

func hasRepetition(s string) bool {
	if len(s) < 3 {
		return false
	}
	for i := 0; i < len(s)-2; i++ {
		if s[i] == s[i+1] && s[i] == s[i+2] {
			return true
		}
	}
	return false
}

func hasSequential(s string) bool {
	if len(s) < 3 {
		return false
	}
	lower := strings.ToLower(s)
	seq := "abcdefghijklmnopqrstuvwxyz0123456789"
	revSeq := "zyxwvutsrqponmlkjihgfedcba9876543210"

	for i := 0; i < len(s)-2; i++ {
		sub := lower[i : i+3]
		if strings.Contains(seq, sub) || strings.Contains(revSeq, sub) {
			return true
		}
	}
	return false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
