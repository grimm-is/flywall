// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package qos

import (
	"testing"
)

func TestCalculateFWMark(t *testing.T) {
	tests := []struct {
		policyIdx int
		classIdx  int
		expected  uint32
	}{
		{0, 0, 0xF000},
		{0, 1, 0xF001},
		{1, 0, 0xF100},
		{1, 5, 0xF105},
		{10, 20, 0xFA14}, // 0xA = 10, 0x14 = 20
		{15, 255, 0xFFFF},
	}

	for _, tt := range tests {
		got := CalculateFWMark(tt.policyIdx, tt.classIdx)
		if got != tt.expected {
			t.Errorf("CalculateFWMark(%d, %d) = 0x%x; want 0x%x", tt.policyIdx, tt.classIdx, got, tt.expected)
		}
	}
}
