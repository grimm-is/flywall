// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package orca

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"grimm.is/flywall/tools/pkg/toolbox/harness"
	"grimm.is/flywall/tools/pkg/toolbox/timeouts"
)

// AgentPort is the vsock port the agent listens on (Linux only)
const AgentPort = 5000

// TestJob represents a test to be run
type TestJob struct {
	ScriptPath string        // Relative to project root (e.g., "t/01-sanity/sanity_test.sh")
	Timeout    time.Duration // Per-test timeout (parsed from TEST_TIMEOUT comment)
	Skip       bool          // If true, test should be skipped (parsed from SKIP=true)
	SkipReason string        // Reason for skipping (from comment after SKIP=true)
	BatchDir   string        // If set, this is a batched directory (all scripts run in one VM)
	Scripts    []string      // For batch jobs: list of scripts to run in sequence
}

// Default timeout for tests that don't specify one
const DefaultTestTimeout = 90 * time.Second

// TestResult represents the outcome of running a test
type TestResult struct {
	Job       TestJob
	Result    string
	Duration  time.Duration
	Error     error
	RawOutput string
	Suite     *harness.TestSuite
	WorkerID  string
	StartTime time.Time
}

// Regex to match TEST_TIMEOUT comment in scripts
var testTimeoutRe = regexp.MustCompile(`(?m)^#?\s*TEST_TIMEOUT[=:]\s*(\d+)`)

// DiscoverTests finds all test scripts in t/ and parses their timeouts.
// Directories containing a BATCH marker file will have all tests run in a single VM.
func DiscoverTests(projectRoot string, target string, history *TestHistory) ([]TestJob, error) {
	var jobs []TestJob

	var testDir string
	if target != "" {
		testDir = filepath.Join(projectRoot, "integration_tests", target)
	} else {
		// Default to integration_tests/linux, fallback to t/ if not found (backwards compatibility)
		testDir = filepath.Join(projectRoot, "integration_tests", "linux")
		if _, err := os.Stat(testDir); os.IsNotExist(err) {
			testDir = filepath.Join(projectRoot, "t")
		}
	}

	// First pass: find batch directories (containing BATCH marker file)
	batchDirs := make(map[string]bool)
	filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == "BATCH" {
			batchDirs[filepath.Dir(path)] = true
		}
		return nil
	})

	// Second pass: collect tests, grouping batch directories
	batchScripts := make(map[string][]string)       // batchDir -> list of scripts
	batchTimeouts := make(map[string]time.Duration) // batchDir -> total timeout

	err := filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, "_test.sh") {
			relPath, _ := filepath.Rel(projectRoot, path)
			dir := filepath.Dir(path)

			// Check if this file is in a batch directory
			if batchDirs[dir] {
				// Add to batch group
				batchScripts[dir] = append(batchScripts[dir], relPath)
				// Accumulate timeout
				timeout := parseTestTimeout(path)
				batchTimeouts[dir] += timeout
				return nil
			}

			// Regular (non-batched) test
			staticTimeout := parseTestTimeout(path)

			var finalTimeout = staticTimeout
			if history != nil {
				avg := history.GetExpectedDuration(relPath)
				if avg > 0 {
					baseDynamic := time.Duration(float64(avg) * 2.5)
					dynamic := timeouts.Scale(baseDynamic)
					if dynamic < 5*time.Second {
						dynamic = 5 * time.Second
					}
					if staticTimeout > dynamic {
						finalTimeout = staticTimeout
					} else {
						finalTimeout = dynamic
					}
				}
			}

			skip, skipReason := parseTestSkip(path)

			jobs = append(jobs, TestJob{
				ScriptPath: relPath,
				Timeout:    finalTimeout,
				Skip:       skip,
				SkipReason: skipReason,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create batch jobs from collected scripts
	for dir, scripts := range batchScripts {
		relDir, _ := filepath.Rel(projectRoot, dir)

		// Sort scripts for deterministic order
		sortedScripts := make([]string, len(scripts))
		copy(sortedScripts, scripts)
		// Use simple sort (alphabetical)
		for i := 0; i < len(sortedScripts)-1; i++ {
			for j := i + 1; j < len(sortedScripts); j++ {
				if sortedScripts[i] > sortedScripts[j] {
					sortedScripts[i], sortedScripts[j] = sortedScripts[j], sortedScripts[i]
				}
			}
		}

		// Total timeout with 30s buffer for VM overhead
		totalTimeout := batchTimeouts[dir] + 30*time.Second

		jobs = append(jobs, TestJob{
			ScriptPath: relDir + "/*", // Display name shows it's a batch
			BatchDir:   relDir,        // Mark as batch
			Scripts:    sortedScripts, // All scripts to run
			Timeout:    totalTimeout,
		})
	}

	return jobs, nil
}

func parseTestTimeout(scriptPath string) time.Duration {
	file, err := os.Open(scriptPath)
	if err != nil {
		return timeouts.Scale(DefaultTestTimeout)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() && lineCount < 50 {
		line := scanner.Text()
		lineCount++

		if match := testTimeoutRe.FindStringSubmatch(line); match != nil {
			if seconds, err := strconv.Atoi(match[1]); err == nil {
				// Apply TIME_DILATION scaling to the timeout
				return timeouts.Scale(time.Duration(seconds) * time.Second)
			}
		}
	}

	return timeouts.Scale(DefaultTestTimeout)
}

// testSkipRe matches SKIP=true with optional comment
var testSkipRe = regexp.MustCompile(`(?m)^\s*SKIP=(true|1|yes)(?:\s*#\s*(.*))?`)

func parseTestSkip(scriptPath string) (bool, string) {
	file, err := os.Open(scriptPath)
	if err != nil {
		return false, ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() && lineCount < 50 {
		line := scanner.Text()
		lineCount++

		if match := testSkipRe.FindStringSubmatch(line); match != nil {
			reason := ""
			if len(match) > 2 {
				reason = strings.TrimSpace(match[2])
			}
			return true, reason
		}
	}

	return false, ""
}
