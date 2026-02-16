// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package qos

// CalculateFWMark generates a unique firewall mark for a class within a policy.
// Format: 0xF<PolicyIdx><ClassIdx> (approximate, depending on bit usage)
// For this implementation: 0xF000 + (policyIdx << 8) + classIdx
// Valid policyIdx: 0-15, Valid classIdx: 0-255
func CalculateFWMark(policyIdx int, classIdx int) uint32 {
	return uint32(0xF000 + (policyIdx << 8) + classIdx)
}
