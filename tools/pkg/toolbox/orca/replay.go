// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package orca

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"grimm.is/flywall/tools/pkg/toolbox/harness"
)

type replayItem struct {
	Path     string
	LogPath  string
	Duration time.Duration
}

// runReplay replays logs from a previous run
func runReplay(args []string) error {
	projectRoot, buildDir := locateBuildDir()

	// Check for help
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		helpReplay()
		return nil
	}

	history, err := LoadHistory(buildDir)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	if len(history.RunMeta) == 0 {
		return fmt.Errorf("no test history found")
	}

	var runID string
	if len(args) > 0 {
		runID = args[0]
	} else {
		// Use latest
		runID = history.RunMeta[len(history.RunMeta)-1].RunID
	}

	fmt.Printf("Replaying run: %s\n", runID)

	// Collect items
	var items []replayItem
	seenPaths := make(map[string]bool)

	for path, stats := range history.Tests {
		for _, exec := range stats.Executions {
			if exec.RunID == runID {
				// Deduplicate: if we already saw this test path for this run, skip
				if seenPaths[path] {
					continue
				}
				seenPaths[path] = true

				logPath := exec.LogPath
				if !filepath.IsAbs(logPath) {
					// Logs are stored relative to build directory in history?
					// Or relative to where orca runs?
					// History stores paths as they were computed.
					// Let's try joining with buildDir first (most likely for test-results)
					candidate := filepath.Join(buildDir, logPath)
					if _, err := os.Stat(candidate); err == nil {
						logPath = candidate
					} else {
						// Fallback to projectRoot
						candidate = filepath.Join(projectRoot, logPath)
						if _, err := os.Stat(candidate); err == nil {
							logPath = candidate
						}
					}
				}

				if _, err := os.Stat(logPath); err == nil {
					items = append(items, replayItem{
						Path:     path,
						LogPath:  logPath,
						Duration: exec.Duration,
					})
				}
			}
		}
	}

	if len(items) == 0 {
		return fmt.Errorf("no logs found for run %s", runID)
	}

	// Print header
	printHeader()
	fmt.Printf("🚀 Replaying %d tests | RunID: %s\n\n", len(items), runID)

	// Use the same UI format as live tests
	// Header
	fmt.Printf("🏁  %-45s [ ✅ | ❌ | 🚧 ] DURATION\n", "TEST NAME")
	fmt.Println(strings.Repeat("─", 85))

	var passed, failed, skipped int
	var failedItems []replayItem

	// Sort items
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})

	for _, item := range items {
		// Parse the log file
		file, err := os.Open(item.LogPath)
		if err != nil {
			fmt.Printf("Failed to open log %s: %v\n", item.LogPath, err)
			continue
		}

		// Create parser and parse
		parser := harness.NewParser(file)
		suite, err := parser.Parse()
		file.Close()

		if err != nil {
			fmt.Printf("Failed to parse log %s: %v\n", item.LogPath, err)
			continue
		}

		// Calculate stats from suite
		p, f, s := suite.Summary()

		// Determine status
		statusIcon := "✅"
		if f > 0 {
			statusIcon = "❌"
			failed++
			failedItems = append(failedItems, item)
		} else if s > 0 && p == 0 {
			statusIcon = "🚧"
			skipped++
		} else {
			passed++
		}

		// Strip prefix and trim test name if too long
		dispName := strings.TrimPrefix(item.Path, "integration_tests/linux/")
		if len(dispName) > 45 {
			dispName = "..." + dispName[len(dispName)-42:]
		}

		durationStr := item.Duration.Round(time.Millisecond).String()

		fmt.Printf("%s  %-45s [ %2d | %2d | %2d ] %s\n",
			statusIcon,
			dispName,
			p,
			f,
			s,
			durationStr,
		)

		// If failed, showing failure details would be nice
		if f > 0 {
			// Find failure reason in suite
			for _, test := range suite.Results {
				if !test.Passed && !test.Skipped {
					// Indent failure message
					fmt.Printf("   └─ %s\n", test.Description)
				}
			}
		}
	}

	fmt.Println()
	if len(failedItems) > 0 {
		fmt.Println("Failed tests:")
		for _, item := range failedItems {
			relPath, _ := filepath.Rel(projectRoot, item.LogPath)
			if relPath == "" {
				relPath = item.LogPath
			}
			fmt.Printf("  ❌ %s\n     └─ %s\n", strings.TrimPrefix(item.Path, "integration_tests/linux/"), relPath)
		}
	}

	return nil
}