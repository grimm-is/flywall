// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package orca

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	DefaultMaxRuns  = 20
	HistoryFileName = "test-history.json"
	FlakyThreshold  = 0.9 // Tests passing < 90% are considered flaky
)

// RunMetadata stores high-level info about a test run
type RunMetadata struct {
	RunID     string    `json:"run_id"`
	Timestamp time.Time `json:"timestamp"`
	Passed    int       `json:"passed"`
	Failed    int       `json:"failed"`
	Skipped   int       `json:"skipped"`
	Workers   int       `json:"workers"`
}

// TestExecution represents a single execution of a test
type TestExecution struct {
	RunID     string        `json:"run_id"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Status    string        `json:"status"`   // "pass", "fail", "skip"
	LogPath   string        `json:"log_path"` // Relative path to log execution
}

// TestStats holds history and aggregate data for a specific test
type TestStats struct {
	Path        string          `json:"path"`
	Executions  []TestExecution `json:"executions"`
	AvgDuration time.Duration   `json:"avg_duration"`

	// Welford's Online Algorithm Fields
	M2    float64 `json:"m2"`    // Sum of squares of differences
	Count int64   `json:"count"` // Total number of samples tracked
}

// TestHistory tracks test results grouped by test path
type TestHistory struct {
	RunMeta []RunMetadata         `json:"run_meta"`
	Tests   map[string]*TestStats `json:"tests"`
	MaxRuns int                   `json:"maxRuns"`
}

// TestHealth represents detailed health statistics for a single test (computed view)
type TestHealth struct {
	TestPath    string
	PassCount   int
	FailCount   int
	SkipCount   int
	TotalRuns   int
	PassRate    float64
	LastRun     time.Time
	LastStatus  string // "pass", "fail", "skip"
	Grade       string // A, B, C, D, F
	Streak      int    // Current streak of passes
	AvgDuration time.Duration
	MaxDuration time.Duration
}

// Old structs for compatibility/migration if needed (we are dropping them though)
// WorkerRun no longer stored directly in history
type WorkerRun struct {
	WorkerID int             `json:"worker_id"`
	Tests    []TestRunResult `json:"tests"`
}

type TestRunResult struct {
	TestPath string        `json:"test"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
	LogPath  string        `json:"log_path"`
}

// LoadHistory loads test history from disk
func LoadHistory(buildDir string) (*TestHistory, error) {
	path := filepath.Join(buildDir, HistoryFileName)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &TestHistory{
			MaxRuns: DefaultMaxRuns,
			Tests:   make(map[string]*TestStats),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read history: %w", err)
	}

	var history TestHistory
	if err := json.Unmarshal(data, &history); err != nil {
		// Fallback: If unmarshal fails, it might be the old format. Return empty.
		// In a real migration we might try to convert, but for dev tool it's okay to reset.
		fmt.Println("Warning: Failed to parse history (possibly old format), starting fresh.")
		return &TestHistory{
			MaxRuns: DefaultMaxRuns,
			Tests:   make(map[string]*TestStats),
		}, nil
	}

	if history.MaxRuns == 0 {
		history.MaxRuns = DefaultMaxRuns
	}
	if history.Tests == nil {
		history.Tests = make(map[string]*TestStats)
	}

	return &history, nil
}

// Save writes history to disk and prunes old logs
func (h *TestHistory) Save(buildDir string) error {
	// Prune RunMeta
	if len(h.RunMeta) > h.MaxRuns {
		h.RunMeta = h.RunMeta[len(h.RunMeta)-h.MaxRuns:]
	}

	// Prune Test Executions and delete logs
	for _, stats := range h.Tests {
		if len(stats.Executions) > h.MaxRuns {
			toRemove := stats.Executions[:len(stats.Executions)-h.MaxRuns]
			stats.Executions = stats.Executions[len(stats.Executions)-h.MaxRuns:]

			// Delete logs for removed executions
			for _, exec := range toRemove {
				if exec.LogPath != "" {
					fullPath := filepath.Join(buildDir, exec.LogPath)
					_ = os.Remove(fullPath)
				}
			}
		}
	}

	path := filepath.Join(buildDir, HistoryFileName)
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// AddRun adds a new test run to history
func (h *TestHistory) AddRun(runID string, passed, failed, skipped int, workers []WorkerRun) {
	// 1. Add Metadata
	meta := RunMetadata{
		RunID:     runID,
		Timestamp: time.Now(),
		Passed:    passed,
		Failed:    failed,
		Skipped:   skipped,
		Workers:   len(workers),
	}
	h.RunMeta = append(h.RunMeta, meta)

	// 2. Process Results
	for _, w := range workers {
		for _, t := range w.Tests {
			stats, ok := h.Tests[t.TestPath]
			if !ok {
				stats = &TestStats{
					Path: t.TestPath,
				}
				h.Tests[t.TestPath] = stats
			}

			exec := TestExecution{
				RunID:     runID,
				Timestamp: meta.Timestamp,
				Duration:  t.Duration,
				Status:    t.Status,
				LogPath:   t.LogPath,
			}
			stats.Executions = append(stats.Executions, exec)

			// Update Stats using Welford's Online Algorithm
			// Only update for Passing tests to key baseline valid
			if t.Status == "pass" {
				val := float64(t.Duration.Nanoseconds())
				stats.Count++

				// Standard Welford:
				// delta = x - mean
				// mean += delta / count
				// delta2 = x - mean
				// M2 += delta * delta2

				currMean := float64(stats.AvgDuration.Nanoseconds())
				delta := val - currMean
				currMean += delta / float64(stats.Count)
				stats.AvgDuration = time.Duration(currMean) // Update mean

				delta2 := val - currMean
				stats.M2 += delta * delta2
			}
		}
	}
}

// GetStdDev returns the standard deviation (in duration)
func (h *TestHistory) GetStdDev(testPath string) time.Duration {
	stats, ok := h.Tests[testPath]
	if !ok || stats.Count < 2 {
		return 0
	}
	variance := stats.M2 / float64(stats.Count-1)
	if variance < 0 {
		return 0
	}
	return time.Duration(math.Sqrt(variance))
}

// IsAnomalous checks if a duration is significantly higher than historical average
// Returns isAnomalous, zScore, expected
func (h *TestHistory) IsAnomalous(testPath string, d time.Duration) (bool, float64, time.Duration) {
	stats, ok := h.Tests[testPath]
	if !ok || stats.Count < 3 { // Need a few samples
		return false, 0, 0
	}

	// Flag if significantly higher OR lower than historical average
	stdDev := h.GetStdDev(testPath)
	if stdDev == 0 {
		return false, 0, stats.AvgDuration
	}

	diff := float64(d - stats.AvgDuration)
	zScore := diff / float64(stdDev)

	// Flag if > 2.5 sigma (approx 99% percentile in normal dist)
	return math.Abs(zScore) > 2.0, zScore, stats.AvgDuration
}

// PrintDetail prints detailed results for a specific run
// index: 0 = latest, 1 = previous, etc.
func (h *TestHistory) PrintDetail(index int) {
	if index < 0 || index >= len(h.RunMeta) {
		fmt.Printf("Invalid history index: %d (Available: 0-%d)\n", index, len(h.RunMeta)-1)
		return
	}

	// Meta is stored Oldest -> Newest?
	// AddRun appends. So index 0 usually means "Latest" in CLI usage, but we need to map it.
	// Let's assume CLI passes 0 for latest.
	// So actual index = len - 1 - index
	actualIndex := len(h.RunMeta) - 1 - index
	if actualIndex < 0 {
		fmt.Printf("History index out of bounds.\n")
		return
	}

	meta := h.RunMeta[actualIndex]
	runID := meta.RunID

	fmt.Printf("\n--- Run Details: %s ---\n", runID)
	fmt.Printf("Time:    %s\n", meta.Timestamp.Format(time.RFC822))
	fmt.Printf("Result:  %d passed, %d failed, %d skipped\n", meta.Passed, meta.Failed, meta.Skipped)
	fmt.Printf("Workers: %d\n\n", meta.Workers)

	// Find executions for this runID
	// This is inefficient (O(Tests * Runs)), but history is small.
	type detailedResult struct {
		Path     string
		Status   string
		Duration time.Duration
		LogPath  string
	}
	var results []detailedResult

	for path, stats := range h.Tests {
		for _, exec := range stats.Executions {
			if exec.RunID == runID {
				results = append(results, detailedResult{
					Path:     path,
					Status:   exec.Status,
					Duration: exec.Duration,
					LogPath:  exec.LogPath,
				})
			}
		}
	}

	// Sort by Status (Fail first), then Path
	sort.Slice(results, func(i, j int) bool {
		score := func(s string) int {
			if s == "fail" {
				return 0
			}
			if s == "skip" {
				return 2
			}
			return 1 // pass
		}
		if score(results[i].Status) != score(results[j].Status) {
			return score(results[i].Status) < score(results[j].Status)
		}
		return results[i].Path < results[j].Path
	})

	for _, r := range results {
		icon := "✅"
		if r.Status == "fail" {
			icon = "❌"
		} else if r.Status == "skip" {
			icon = "🚧 "
		}

		extra := ""
		if isAnom, _, expected := h.IsAnomalous(r.Path, r.Duration); isAnom && expected > 0 {
			pct := float64(r.Duration-expected) / float64(expected) * 100
			if pct > 0 {
				extra = fmt.Sprintf(" 🐢 +%.0f%%", pct)
			} else {
				extra = fmt.Sprintf(" 🐇 %.0f%%", pct)
			}
		}

		fmt.Printf("%s %-55s %s%s\n", icon, r.Path, r.Duration.Round(time.Millisecond), extra)
		if r.Status == "fail" && r.LogPath != "" {
			fmt.Printf("   └─ Log: %s\n", r.LogPath)
		}
	}
}

// GetExpectedDuration returns the average duration for a test
func (h *TestHistory) GetExpectedDuration(testPath string) time.Duration {
	if stats, ok := h.Tests[testPath]; ok {
		return stats.AvgDuration
	}
	return 0
}

// GetStreak returns the current passing streak for a test
func (h *TestHistory) GetStreak(testPath string) int {
	stats, ok := h.Tests[testPath]
	if !ok || len(stats.Executions) == 0 {
		return 0
	}

	streak := 0
	// Iterate from newest (end) to oldest
	for i := len(stats.Executions) - 1; i >= 0; i-- {
		if stats.Executions[i].Status == "pass" {
			streak++
		} else {
			break
		}
	}
	return streak
}

// PrintFlakyReport prints a report of flaky tests
// filter: optional list of test paths to include (nil shows all)
func (h *TestHistory) PrintFlakyReport(filter []string) {
	health := h.CalculateTestHealth()
	var flaky []TestHealth

	// Create filter map for O(1) lookup
	allow := make(map[string]bool)
	for _, f := range filter {
		allow[f] = true
	}

	for _, t := range health {
		// Filter
		if len(filter) > 0 && !allow[t.TestPath] {
			continue
		}

		if t.PassCount > 0 && t.FailCount > 0 {
			flaky = append(flaky, t)
		}
	}

	if len(flaky) == 0 {
		return
	}

	fmt.Println("\n--- Flaky Tests ---")
	for _, s := range flaky {
		status := "flaky"
		if s.PassRate >= 0.8 {
			status = "occasional fail"
		} else if s.PassRate < 0.5 {
			status = "mostly failing"
		}
		fmt.Printf("  %-55s %d/%d pass (%s)\n", s.TestPath, s.PassCount, s.TotalRuns, status)
	}
}

// CalculateTestHealth analyzes test history and returns health metrics
func (h *TestHistory) CalculateTestHealth() []TestHealth {
	var health []TestHealth

	for path, stats := range h.Tests {
		hStat := TestHealth{
			TestPath:    path,
			AvgDuration: stats.AvgDuration,
		}

		for _, exec := range stats.Executions {
			hStat.TotalRuns++
			if exec.Status == "pass" {
				hStat.PassCount++
				// Update streak
				if hStat.LastStatus != "fail" && hStat.LastStatus != "skip" {
					// Simplified streak logic: this iterates oldest to newest?
					// stats.Executions is appended, so Oldest -> Newest.
					// We need to calc streak from newest backwards or just track it.
				}
				if exec.Duration > hStat.MaxDuration {
					hStat.MaxDuration = exec.Duration
				}
			} else if exec.Status == "fail" {
				hStat.FailCount++
			} else if exec.Status == "skip" {
				hStat.SkipCount++
			}
		}

		// Recalculate streak properly (Newest -> Oldest)
		streak := 0
		for i := len(stats.Executions) - 1; i >= 0; i-- {
			if stats.Executions[i].Status == "pass" {
				streak++
			} else {
				break
			}
		}
		hStat.Streak = streak

		if len(stats.Executions) > 0 {
			last := stats.Executions[len(stats.Executions)-1]
			hStat.LastRun = last.Timestamp
			hStat.LastStatus = last.Status
		}

		if hStat.TotalRuns > 0 {
			hStat.PassRate = float64(hStat.PassCount) / float64(hStat.TotalRuns)
		}

		// Calculate Grade
		if hStat.TotalRuns > 0 {
			if hStat.PassRate >= 0.95 {
				hStat.Grade = "A"
			} else if hStat.PassRate >= 0.80 {
				hStat.Grade = "B"
			} else if hStat.PassRate >= 0.50 {
				hStat.Grade = "C"
			} else if hStat.PassRate >= 0.20 {
				hStat.Grade = "D"
			} else {
				hStat.Grade = "F"
			}
		} else {
			hStat.Grade = "?"
		}

		health = append(health, hStat)
	}

	// Sort by Grade (F first), then PassRate
	sort.Slice(health, func(i, j int) bool {
		val := func(g string) int {
			switch g {
			case "F":
				return 0
			case "D":
				return 1
			case "C":
				return 2
			case "?":
				return 3
			case "B":
				return 4
			case "A":
				return 5
			}
			return 6
		}
		if val(health[i].Grade) != val(health[j].Grade) {
			return val(health[i].Grade) < val(health[j].Grade)
		}
		return health[i].PassRate < health[j].PassRate
	})

	return health
}

// PrintSummary prints a summary of the test history
// filter: optional list of test paths to include (nil shows all)
func (h *TestHistory) PrintSummary(limit int, filter []string) {
	fmt.Printf("\n--- Test History Summary ---\n")
	health := h.CalculateTestHealth()

	// Create filter map for O(1) lookup
	allow := make(map[string]bool)
	for _, f := range filter {
		allow[f] = true
	}

	fmt.Printf("%-55s %-5s %-10s %-10s %-15s %-10s\n",
		"Test", "Grade", "Pass/Run", "Rate", "Last Run", "Streak")

	for _, t := range health {
		// Filter
		if len(filter) > 0 && !allow[t.TestPath] {
			continue
		}

		// Grade Color
		gradeStr := t.Grade

		// Last Run relative time
		timeStr := ""
		if t.LastRun.IsZero() {
			timeStr = "never"
		} else {
			since := time.Since(t.LastRun).Round(time.Minute)
			timeStr = fmt.Sprintf("%v ago", since)
			if since < time.Minute {
				timeStr = "just now"
			}
		}

		// Status icon
		statusIcon := "✅"
		if t.LastStatus == "fail" {
			statusIcon = "❌"
		} else if t.LastStatus == "skip" {
			statusIcon = "⏭ "
		} else if t.LastStatus == "pending" {
			statusIcon = "⚪️"
		}

		avgStr := t.AvgDuration.Round(time.Millisecond).String()

		fmt.Printf("%s %-53s %-5s %3d/%-6d %-10.0f %-15s %d (Avg: %s)\n",
			statusIcon, t.TestPath, gradeStr, t.PassCount, t.TotalRuns, t.PassRate*100, timeStr, t.Streak, avgStr)
	}
}
