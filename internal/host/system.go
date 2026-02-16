// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package host

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MemoryInfo holds system memory statistics.
type MemoryInfo struct {
	TotalBytes     uint64
	FreeBytes      uint64
	AvailableBytes uint64
}

// GetMemoryInfo reads and parses /proc/meminfo.
func GetMemoryInfo() (*MemoryInfo, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &MemoryInfo{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Field format: "Key: VALUE kB"
		// value is fields[1]
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		valBytes := val * 1024

		switch fields[0] {
		case "MemTotal:":
			info.TotalBytes = valBytes
		case "MemFree:":
			info.FreeBytes = valBytes
		case "MemAvailable:":
			info.AvailableBytes = valBytes
		}
	}

	// Fallback for Available if not present (older kernels)
	if info.AvailableBytes == 0 {
		info.AvailableBytes = info.FreeBytes
	}

	return info, nil
}

// CheckBPFJIT checks if eBPF JIT is enabled.
func CheckBPFJIT() (bool, error) {
	jitEnabled, err := os.ReadFile("/proc/sys/net/core/bpf_jit_enable")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(jitEnabled)) == "1", nil
}

// GetBPFJITLimit returns the eBPF JIT memory limit in MB.
func GetBPFJITLimit() (int64, error) {
	jitLimit, err := os.ReadFile("/proc/sys/net/core/bpf_jit_limit")
	if err != nil {
		return 0, err
	}

	var limit int64
	_, err = fmt.Sscanf(strings.TrimSpace(string(jitLimit)), "%d", &limit)
	if err != nil {
		return 0, err
	}

	return limit / 1024 / 1024, nil
}

// SetBPFJITLimit sets the eBPF JIT memory limit in MB.
func SetBPFJITLimit(limitMB int64) error {
	limitBytes := limitMB * 1024 * 1024
	data := fmt.Sprintf("%d", limitBytes)
	return os.WriteFile("/proc/sys/net/core/bpf_jit_limit", []byte(data), 0644)
}

// GetDeviceID returns a unique identifier for this system.
// It tries to read the hardware UUID from /sys/class/dmi/id/product_uuid.
func GetDeviceID() string {
	// Try hardware UUID first
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	// Try /etc/machine-id
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	return "unknown-device"
}

// SystemRequirementError represents a missing system requirement.
type SystemRequirementError struct {
	Feature string
	Message string
	Fatal   bool
}

func (e *SystemRequirementError) Error() string {
	return fmt.Sprintf("%s: %s", e.Feature, e.Message)
}

// VerifyBPFSupport checks if the system meets requirements for eBPF.
func VerifyBPFSupport() []SystemRequirementError {
	var errors []SystemRequirementError

	// 1. Check if /proc/sys/net/core/bpf_jit_enable exists
	if _, err := os.Stat("/proc/sys/net/core/bpf_jit_enable"); os.IsNotExist(err) {
		errors = append(errors, SystemRequirementError{
			Feature: "eBPF",
			Message: "Kernel does not support eBPF JIT",
			Fatal:   true,
		})
		return errors // Fatal, no point checking others
	}

	// 2. Check JIT status
	enabled, err := CheckBPFJIT()
	if err != nil || !enabled {
		errors = append(errors, SystemRequirementError{
			Feature: "JIT",
			Message: "eBPF JIT is not enabled",
			Fatal:   false, // Technically works, but slow
		})
	}

	// 3. Check JIT limit
	limit, err := GetBPFJITLimit()
	if err == nil && limit < 256 {
		errors = append(errors, SystemRequirementError{
			Feature: "JIT Limit",
			Message: fmt.Sprintf("eBPF JIT limit too low (%d MB, recommended >= 256 MB)", limit),
			Fatal:   false,
		})
	}

	// 4. Check memory
	if mem, err := GetMemoryInfo(); err == nil {
		if mem.AvailableBytes < 512*1024*1024 {
			errors = append(errors, SystemRequirementError{
				Feature: "Memory",
				Message: fmt.Sprintf("Low available memory (%d MB, recommended >= 512 MB)", mem.AvailableBytes/1024/1024),
				Fatal:   false,
			})
		}
	}

	return errors
}
