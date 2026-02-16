// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package orca

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"grimm.is/flywall/tools/pkg/protocol"
	"grimm.is/flywall/tools/pkg/toolbox/harness"
	"grimm.is/flywall/tools/pkg/toolbox/orca/client"
)

// --- Styles ---

var (
	focusedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	unfocusedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	appStyle = lipgloss.NewStyle().Margin(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AEAFAD")).
				MarginTop(1)
)

// --- Items ---

type runItem struct {
	runID     string
	timestamp time.Time
	passed    int
	failed    int
	skipped   int
}

func (i runItem) Title() string {
	timeStr := i.timestamp.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("%s - %s", timeStr, i.runID)
}

func (i runItem) Description() string {
	return fmt.Sprintf("Passed: %d, Failed: %d, Skipped: %d", i.passed, i.failed, i.skipped)
}

func (i runItem) FilterValue() string { return i.runID }

type testItem struct {
	path          string
	status        string
	duration      time.Duration
	logPath       string
	statsLoaded   bool
	passed        int
	failed        int
	skipped       int
	failureReason string
	workerID      string
}

func (i testItem) Title() string {
	marker := "✅"
	if i.status == "fail" {
		marker = "❌"
	} else if i.status == "skip" {
		marker = "🚧"
	} else if i.status == "pending" {
		marker = "⚪️"
	} else if i.status == "running" {
		marker = "⏳"
	}

	// Dynamic padding based on pane width?
	// For now, static to match the requested view format
	displayName := formatDisplayName(i.path, "", "")

	passInt := i.passed
	failInt := i.failed
	skipInt := i.skipped

	durStr := i.duration.Round(time.Millisecond).String()
	if i.duration == 0 {
		durStr = "--"
	}

	return fmt.Sprintf("%-2s %s [%3d |%3d |%3d ] %-9s",
		marker, padRight(displayName, 45), passInt, failInt, skipInt, durStr)
}

func (i testItem) Description() string {
	return ""
}

func (i testItem) FilterValue() string { return i.path }

type healthItem struct {
	TestHealth
}

func (i healthItem) Title() string {
	grade := i.Grade
	icon := "✅"
	if grade == "F" || grade == "D" {
		icon = "❌"
	} else if grade == "?" {
		icon = "❓"
	}

	disp := formatDisplayName(i.TestPath, "", "")
	return fmt.Sprintf("%s %s (Grade: %s)", icon, disp, grade)
}

func (i healthItem) Description() string {
	passRate := i.PassRate * 100
	return fmt.Sprintf("Pass: %.0f%% | Avg: %s | Last: %s (%s)",
		passRate, i.AvgDuration.Round(time.Millisecond), i.LastStatus, i.LastRun.Format("01/02 15:04"))
}

func (i healthItem) FilterValue() string { return i.TestPath }

// --- Model ---

type viewState int
type pane int

const (
	viewRunList viewState = iota
	viewDualPane
	viewHealth
	viewTestHistory
	viewLiveRun
)

const (
	paneList pane = iota
	paneLog
)

type model struct {
	// State
	view       viewState
	activePane pane
	width      int
	height     int
	ready      bool
	err        error

	// Data
	history      *TestHistory
	projectRoot  string
	buildDir     string
	selectedRun  string
	selectedTest string
	liveRunID    string

	// Components
	runList    list.Model     // Initial selection
	testList   list.Model     // Left pane
	logView    viewport.Model // Right pane
	healthList list.Model     // Tab view
	eventChan  chan tea.Msg
}

type testStartedMsg struct {
	name, path string
}

type testOutputMsg struct {
	name, line string
}

type testProgressMsg struct {
	progress protocol.TestProgress
}

type testResultMsg struct {
	result protocol.TestResult
}

type runFinishedMsg struct{ err error }

type testFinishedMsg struct{ err error }

func runTestCmd(path string) tea.Cmd {
	exe, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return testFinishedMsg{err} }
	}
	c := exec.Command(exe, "orca", "test", path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return testFinishedMsg{err}
	})
}

func runTUI(args []string) error {
	projectRoot, buildDir := locateBuildDir()
	history, err := LoadHistory(buildDir)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	if len(history.RunMeta) == 0 {
		return fmt.Errorf("no test history found")
	}

	// 1. Run List
	var runItems []list.Item
	for i := len(history.RunMeta) - 1; i >= 0; i-- {
		meta := history.RunMeta[i]
		runItems = append(runItems, runItem{
			runID:     meta.RunID,
			timestamp: meta.Timestamp,
			passed:    meta.Passed,
			failed:    meta.Failed,
			skipped:   meta.Skipped,
		})
	}
	runDelegate := list.NewDefaultDelegate()
	runDelegate.SetSpacing(0)
	rl := list.New(runItems, runDelegate, 0, 0)
	rl.Title = "Test Runs"
	rl.Styles.Title = titleStyle

	// 2. Test List (Left Pane)
	testDelegate := list.NewDefaultDelegate()
	testDelegate.ShowDescription = false
	testDelegate.SetSpacing(0)
	testDelegate.SetHeight(1)
	testDelegate.Styles.NormalTitle.Padding(0, 1)
	testDelegate.Styles.SelectedTitle.Padding(0, 1)
	testDelegate.Styles.SelectedTitle.Border(lipgloss.NormalBorder(), false, false, false, true)
	testDelegate.Styles.SelectedTitle.BorderForeground(lipgloss.Color("62"))

	tl := list.New([]list.Item{}, testDelegate, 0, 0)
	tl.Title = "Tests"
	tl.SetShowHelp(false)
	tl.SetShowStatusBar(false) // Save space
	tl.Styles.Title = titleStyle

	// 3. Health List
	healthDelegate := list.NewDefaultDelegate()
	healthDelegate.SetSpacing(0)
	hl := list.New([]list.Item{}, healthDelegate, 0, 0)
	hl.Title = "Test Health"
	hl.Styles.Title = titleStyle

	// 4. Log View (Right Pane)
	vp := viewport.New(0, 0)

	m := model{
		view:        viewRunList,
		activePane:  paneList,
		history:     history,
		projectRoot: projectRoot,
		buildDir:    buildDir,
		runList:     rl,
		testList:    tl,
		logView:     vp,
		healthList:  hl,
		eventChan:   make(chan tea.Msg, 100),
	}
	m.loadHealth()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

type liveRunMsg struct {
	tests []TestJob
	err   error
}

func startLiveRunCmd(projectRoot string, history *TestHistory) tea.Cmd {
	return func() tea.Msg {
		tests, err := DiscoverTests(projectRoot, "", history)
		if err != nil {
			return liveRunMsg{err: err}
		}
		return liveRunMsg{tests: tests}
	}
}

func waitForEvent(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

func (m model) Init() tea.Cmd {
	return tea.EnableMouseCellMotion
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
			if m.view == viewDualPane || m.view == viewHealth {
				m.view = viewRunList
				m.reloadHistory()
				return m, nil
			}
			if m.view == viewTestHistory {
				m.view = viewHealth
				m.loadHealth()
				return m, nil
			}
		}

		if m.view == viewDualPane || m.view == viewTestHistory {
			// Layout constants mirroring Update(WindowSizeMsg)
			listWidth := int(float64(m.width) * 0.45)

			// Handle Focus / Click
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				if msg.X < listWidth+2 { // +2 for border/padding approx
					m.activePane = paneList
					// Try to select item
					// Offset: 3 (Border + Title + Margin?)
					row := msg.Y - 3
					if row >= 0 {
						start := m.testList.Index() - m.testList.Cursor()
						target := start + row
						if target >= 0 && target < len(m.testList.Items()) {
							m.testList.Select(target)
							// Trigger log load immediately
							if i, ok := m.testList.SelectedItem().(testItem); ok {
								m.loadLog(i.logPath, false)
							}
						}
					}
				} else {
					m.activePane = paneLog
				}
			}

			// Handle Scroll
			if msg.Type == tea.MouseWheelUp {
				if msg.X < listWidth+2 {
					m.testList.CursorUp()
				} else {
					m.logView.LineUp(1)
				}
			} else if msg.Type == tea.MouseWheelDown {
				if msg.X < listWidth+2 {
					m.testList.CursorDown()
				} else {
					m.logView.LineDown(1)
				}
			}

		} else if m.view == viewRunList {
			// Run List Scroll/Click
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				// Offset: 1 (Margin) + 2 (Title) = 5 approx
				row := msg.Y - 5
				if row >= 0 {
					start := m.runList.Index() - m.runList.Cursor()
					// Items are 2 lines high (Title + Description)
					target := start + (row / 2)
					if target >= 0 && target < len(m.runList.Items()) {
						m.runList.Select(target)

						// Open Dual Pane
						if i, ok := m.runList.SelectedItem().(runItem); ok {
							m.selectedRun = i.runID
							m.view = viewDualPane
							m.activePane = paneList
							m.loadTestsForRun(i.runID)
							if len(m.testList.Items()) > 0 {
								if ti, ok := m.testList.Items()[0].(testItem); ok {
									m.loadLog(ti.logPath, false)
								}
							}
						}
					}
				}
			}

			if msg.Type == tea.MouseWheelUp {
				m.runList.CursorUp()
			} else if msg.Type == tea.MouseWheelDown {
				m.runList.CursorDown()
			}
		} else if m.view == viewHealth {
			// Health List Scroll/Click
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				row := msg.Y - 5
				if row >= 0 {
					start := m.healthList.Index() - m.healthList.Cursor()
					target := start + (row / 2)
					if target >= 0 && target < len(m.healthList.Items()) {
						m.healthList.Select(target)

						// Open Test History
						if i, ok := m.healthList.SelectedItem().(healthItem); ok {
							m.selectedTest = i.TestPath
							m.view = viewTestHistory
							m.activePane = paneList
							m.loadTestHistory(i.TestPath)
							if len(m.testList.Items()) > 0 {
								if ti, ok := m.testList.Items()[0].(testItem); ok {
									m.loadLog(ti.logPath, false)
								}
							}
						}
					}
				}
			}
			if msg.Type == tea.MouseWheelUp {
				m.healthList.CursorUp()
			} else if msg.Type == tea.MouseWheelDown {
				m.healthList.CursorDown()
			}
		}

	case runFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		m.testList.Title = fmt.Sprintf("Live Run (Completed) - %s", m.liveRunID)
		m.saveLiveRunHistory()
		m.reloadHistory()
		return m, waitForEvent(m.eventChan)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Layout logic
		// RunList / HealthList get full size
		h, v := appStyle.GetFrameSize()
		m.runList.SetSize(msg.Width-h, msg.Height-v)
		m.healthList.SetSize(msg.Width-h, msg.Height-v)

		// Dual Pane
		// List gets 45% width, Log gets 55%
		listWidth := int(float64(msg.Width) * 0.45)
		logWidth := msg.Width - listWidth - 4 // borders/margin

		m.testList.SetSize(listWidth, msg.Height-4) // -4 for header/borders
		m.logView.Width = logWidth
		m.logView.Height = msg.Height - 4

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Allow quit if not in log pane (where q might be scrolling? no viewport uses j/k)
			// But allow q everywhere for consistency unless filtering
			if m.view != viewDualPane || m.activePane == paneList || m.testList.FilterState() != list.Filtering {
				return m, tea.Quit
			}

		case "tab":
			if m.view == viewRunList {
				m.view = viewHealth
				return m, nil
			} else if m.view == viewHealth {
				m.view = viewRunList
				return m, nil
			}
			if m.view == viewDualPane || m.view == viewTestHistory {
				// Switch focus
				if m.activePane == paneList {
					m.activePane = paneLog
				} else {
					m.activePane = paneList
				}
				return m, nil
			}

		case "left", "right":
			if (m.view == viewDualPane || m.view == viewTestHistory) && m.testList.FilterState() != list.Filtering {
				if m.activePane == paneList {
					m.activePane = paneLog
				} else {
					m.activePane = paneList
				}
				return m, nil
			}

		case "esc":
			if m.view == viewDualPane {
				m.view = viewRunList
				m.reloadHistory() // Refresh data
				return m, nil
			}
			if m.view == viewHealth {
				m.view = viewRunList
				return m, nil
			}
			if m.view == viewTestHistory {
				m.view = viewHealth
				return m, nil
			}

		case "enter":
			if m.view == viewRunList {
				i, ok := m.runList.SelectedItem().(runItem)
				if ok {
					m.selectedRun = i.runID
					m.view = viewDualPane
					m.activePane = paneList // Reset focus
					m.loadTestsForRun(i.runID)
					// Load log for first item
					if len(m.testList.Items()) > 0 {
						if ti, ok := m.testList.Items()[0].(testItem); ok {
							m.loadLog(ti.logPath, false)
						}
					}
				}
			} else if m.view == viewHealth {
				i, ok := m.healthList.SelectedItem().(healthItem)
				if ok {
					m.selectedTest = i.TestPath
					m.view = viewTestHistory
					m.activePane = paneList
					m.loadTestHistory(i.TestPath)
					if len(m.testList.Items()) > 0 {
						if ti, ok := m.testList.Items()[0].(testItem); ok {
							m.loadLog(ti.logPath, false)
						}
					}
				}
			} else if m.view == viewDualPane {
				// If in list, maybe enter runs test? Or focuses log?
				// User said: "navigate between panes to scroll".
				// Let's make Enter run the test if in list pane.
				if m.activePane == paneList {
					i, ok := m.testList.SelectedItem().(testItem)
					if ok {
						return m, runTestCmd(i.path)
					}
				}
			}

		case "r":
			if m.view == viewDualPane && m.activePane == paneList && m.testList.FilterState() != list.Filtering {
				i, ok := m.testList.SelectedItem().(testItem)
				if ok {
					return m, runTestCmd(i.path)
				}
			}
			if m.view == viewHealth {
				i, ok := m.healthList.SelectedItem().(healthItem)
				if ok {
					return m, runTestCmd(i.TestPath)
				}
			}
			if m.view == viewTestHistory {
				return m, runTestCmd(m.selectedTest)
			}
		case "R":
			if m.view == viewRunList || m.view == viewHealth {
				return m, startLiveRunCmd(m.projectRoot, m.history)
			}
		}

	case liveRunMsg:
		if msg.err != nil {
			return m, nil
		}
		m.view = viewDualPane
		m.activePane = paneList
		m.testList.Title = "Live Run (Running...)"

		var items []list.Item
		for _, t := range msg.tests {
			items = append(items, testItem{path: t.ScriptPath, status: "pending"})
		}
		m.testList.SetItems(items)

		runID := fmt.Sprintf("live-%d", time.Now().Unix())
		m.liveRunID = runID

		go func() {
			client.EnsureServer(false, 0, 0, false, "")
			var infos []client.TestInfo
			for _, t := range msg.tests {
				infos = append(infos, client.TestInfo{
					Path:    t.ScriptPath,
					Timeout: t.Timeout,
					Scripts: t.Scripts,
				})
			}
			logDir := filepath.Join(m.buildDir, "test-results")
			err := client.RunTests(runID, infos, logDir, nil, 0, "",
				func(name, path string) { m.eventChan <- testStartedMsg{name, path} },
				func(name string, line []byte) { m.eventChan <- testOutputMsg{name, string(line)} },
				func(p protocol.TestProgress) { m.eventChan <- testProgressMsg{p} },
				func(r protocol.TestResult) { m.eventChan <- testResultMsg{r} },
			)
			m.eventChan <- runFinishedMsg{err}
		}()
		return m, waitForEvent(m.eventChan)

	case testProgressMsg:
		items := m.testList.Items()
		for i, it := range items {
			if ti, ok := it.(testItem); ok && ti.path == msg.progress.Name {
				ti.passed = msg.progress.Passed
				ti.failed = msg.progress.Failed
				ti.skipped = msg.progress.Skipped
				ti.statsLoaded = true
				items[i] = ti
				m.testList.SetItem(i, ti)
				break
			}
		}
		return m, waitForEvent(m.eventChan)

	case testOutputMsg:
		// If actively viewing this test, tail the log
		if m.view == viewDualPane && m.activePane == paneList {
			if i, ok := m.testList.SelectedItem().(testItem); ok && i.path == msg.name {
				if i.logPath != "" {
					m.loadLog(i.logPath, true)
				}
			}
		}
		return m, waitForEvent(m.eventChan)

	case testStartedMsg:
		items := m.testList.Items()
		for i, it := range items {
			if ti, ok := it.(testItem); ok && ti.path == msg.name {
				ti.status = "running"
				ti.logPath = msg.path
				// Select running test? Maybe annoying if user navigating
				items[i] = ti
				m.testList.SetItem(i, ti)
				break
			}
		}
		return m, waitForEvent(m.eventChan)

	case testResultMsg:
		items := m.testList.Items()
		for i, it := range items {
			if ti, ok := it.(testItem); ok && ti.path == msg.result.Name {
				ti.status = "pass"
				if !msg.result.Passed {
					ti.status = "fail"
				} else if msg.result.Skipped == msg.result.Total {
					ti.status = "skip"
				}
				ti.duration = msg.result.Duration
				ti.passed = msg.result.TasksPassed
				ti.failed = msg.result.TasksFailed
				ti.skipped = msg.result.TasksSkipped
				ti.statsLoaded = true
				ti.workerID = msg.result.WorkerID

				// Find failure reason
				// We don't have suite here, only result. Diagnostics?
				// result.Diagnostics might help.

				items[i] = ti
				m.testList.SetItem(i, ti)
				break
			}
		}
		return m, waitForEvent(m.eventChan)

	case testFinishedMsg:
		m.reloadHistory()
		m.loadHealth()
		if m.view == viewDualPane {
			// Reload current run
			m.loadTestsForRun(m.selectedRun)
			// Reload log if selected test matches
			i, ok := m.testList.SelectedItem().(testItem)
			if ok {
				m.loadLog(i.logPath, false)
			}
		}
		return m, nil
	}

	// Update Components
	switch m.view {
	case viewRunList:
		m.runList, cmd = m.runList.Update(msg)
		cmds = append(cmds, cmd)

	case viewHealth:
		m.healthList, cmd = m.healthList.Update(msg)
		cmds = append(cmds, cmd)

	case viewDualPane, viewTestHistory:
		// Handle Updates based on focus
		if m.activePane == paneList {
			prevSel := m.testList.Index()
			m.testList, cmd = m.testList.Update(msg)
			cmds = append(cmds, cmd)

			// If selection changed, update log
			if m.testList.Index() != prevSel {
				if i, ok := m.testList.SelectedItem().(testItem); ok {
					m.loadLog(i.logPath, false)
				}
			}
		} else {
			m.logView, cmd = m.logView.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	switch m.view {
	case viewRunList:
		return appStyle.Render(m.runList.View())
	case viewHealth:
		return appStyle.Render(m.healthList.View())
	case viewDualPane, viewTestHistory:
		// Left Pane
		var leftView string
		if m.activePane == paneList {
			leftView = focusedStyle.Render(m.testList.View())
		} else {
			leftView = unfocusedStyle.Render(m.testList.View())
		}

		// Right Pane
		var rightView string
		logContent := m.logView.View()

		// Header for log
		logHeader := "Log Output"
		if i, ok := m.testList.SelectedItem().(testItem); ok {
			logHeader = fmt.Sprintf("Log: %s", filepath.Base(i.path))
		}

		logStyle := unfocusedStyle
		if m.activePane == paneLog {
			logStyle = focusedStyle
		}

		// Ensure height consistency
		// viewport view doesn't include borders, style does.
		rightView = logStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				titleStyle.Render(logHeader),
				logContent,
			),
		)

		return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
	}
	return ""
}

// Helpers (Same as before)

func (m *model) reloadHistory() {
	h, err := LoadHistory(m.buildDir)
	if err == nil {
		m.history = h
		var items []list.Item
		for i := len(h.RunMeta) - 1; i >= 0; i-- {
			meta := h.RunMeta[i]
			items = append(items, runItem{
				runID:     meta.RunID,
				timestamp: meta.Timestamp,
				passed:    meta.Passed,
				failed:    meta.Failed,
				skipped:   meta.Skipped,
			})
		}
		m.runList.SetItems(items)
	}
}

func (m *model) loadHealth() {
	health := m.history.CalculateTestHealth()
	var items []list.Item
	for _, h := range health {
		items = append(items, healthItem{h})
	}
	m.healthList.SetItems(items)
}

func (m *model) loadTestsForRun(runID string) {
	var items []list.Item
	seenPaths := make(map[string]bool)

	for path, stats := range m.history.Tests {
		for _, exec := range stats.Executions {
			if exec.RunID == runID {
				if seenPaths[path] {
					continue
				}
				seenPaths[path] = true

				p, f, s := 0, 0, 0
				reason := ""

				fullPath := exec.LogPath
				if !filepath.IsAbs(exec.LogPath) {
					candidate := filepath.Join(m.buildDir, exec.LogPath)
					if _, err := os.Stat(candidate); err == nil {
						fullPath = candidate
					} else {
						fullPath = filepath.Join(m.projectRoot, exec.LogPath)
					}
				}

				if file, err := os.Open(fullPath); err == nil {
					parser := harness.NewParser(file)
					if suite, err := parser.Parse(); err == nil {
						p, f, s = suite.Summary()
						if f > 0 {
							for _, res := range suite.Results {
								if !res.Passed && !res.Skipped {
									reason = res.Description
									break
								}
							}
						}
					}
					file.Close()
				}

				items = append(items, testItem{
					path:          path,
					status:        exec.Status,
					duration:      exec.Duration,
					logPath:       exec.LogPath,
					statsLoaded:   true,
					passed:        p,
					failed:        f,
					skipped:       s,
					failureReason: reason,
				})
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		ti := items[i].(testItem)
		tj := items[j].(testItem)

		score := func(s string) int {
			if s == "fail" {
				return 0
			}
			if s == "skip" {
				return 2
			}
			return 1
		}

		si := score(ti.status)
		sj := score(tj.status)

		if si != sj {
			return si < sj
		}
		return ti.path < tj.path
	})

	m.testList.SetItems(items)
	m.testList.ResetSelected()
	m.testList.Title = "Tests: " + runID
}

func cleanLogContent(data []byte) string {
	s := string(data)
	// Strip ANSI escape codes
	// Matches CSI sequences like \x1b[31m or \x1b[K
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	s = re.ReplaceAllString(s, "")

	// Normalize carriage returns to newlines to unroll progress bars
	s = strings.ReplaceAll(s, "\r", "\n")

	// Apply TAP Highlighting
	s = harness.HighlightTAPBlock(s)

	return s
}

func (m *model) loadLog(logPath string, tail bool) {
	fullPath := logPath
	if !filepath.IsAbs(logPath) {
		candidate := filepath.Join(m.buildDir, logPath)
		if _, err := os.Stat(candidate); err == nil {
			fullPath = candidate
		} else {
			fullPath = filepath.Join(m.projectRoot, logPath)
		}
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		m.logView.SetContent(fmt.Sprintf("Failed to read log: %v", err))
		return
	}

	m.logView.SetContent(cleanLogContent(content))
	if tail {
		m.logView.GotoBottom()
	} else {
		m.logView.GotoTop()
	}
}

func (m *model) saveLiveRunHistory() {
	workerRuns := make(map[string]*WorkerRun)
	passed, failed, skipped := 0, 0, 0

	for _, it := range m.testList.Items() {
		if ti, ok := it.(testItem); ok {
			if ti.workerID == "" {
				ti.workerID = "0"
			}
			wid := ti.workerID
			wIDInt := 0
			if val, err := strconv.Atoi(wid); err == nil {
				wIDInt = val
			}

			wr, ok := workerRuns[wid]
			if !ok {
				wr = &WorkerRun{WorkerID: wIDInt}
				workerRuns[wid] = wr
			}

			if ti.status == "pass" {
				passed++
			}
			if ti.status == "fail" {
				failed++
			}
			if ti.status == "skip" {
				skipped++
			}

			// Relativize paths
			scriptPath := strings.TrimPrefix(ti.path, m.projectRoot+"/")
			logPath := strings.TrimPrefix(ti.logPath, m.buildDir+"/")

			wr.Tests = append(wr.Tests, TestRunResult{
				TestPath: scriptPath,
				Status:   ti.status,
				Duration: ti.duration,
				LogPath:  logPath,
			})
		}
	}

	var workers []WorkerRun
	for _, wr := range workerRuns {
		workers = append(workers, *wr)
	}

	m.history.AddRun(m.liveRunID, passed, failed, skipped, workers)
	m.history.Save(m.buildDir)
}

func (m *model) loadTestHistory(testPath string) {
	stats, ok := m.history.Tests[testPath]
	if !ok {
		m.testList.SetItems([]list.Item{})
		m.testList.Title = "History: " + testPath
		return
	}

	var items []list.Item
	// Iterate backwards (Newest first)
	for i := len(stats.Executions) - 1; i >= 0; i-- {
		exec := stats.Executions[i]

		// Map Status to int counts for display (hacky but reuses testItem)
		p, f, s := 0, 0, 0
		if exec.Status == "pass" {
			p = 1
		}
		if exec.Status == "fail" {
			f = 1
		}
		if exec.Status == "skip" {
			s = 1
		}

		items = append(items, testItem{
			path:        exec.RunID, // Display Run ID
			status:      exec.Status,
			duration:    exec.Duration,
			logPath:     exec.LogPath,
			statsLoaded: true,
			passed:      p,
			failed:      f,
			skipped:     s,
		})
	}

	m.testList.SetItems(items)
	m.testList.ResetSelected()
	m.testList.Title = "History: " + filepath.Base(testPath)
}
