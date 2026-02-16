// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/google/uuid"
	"grimm.is/flywall/tools/pkg/protocol"
)

// GetOrcaPaths returns the socket and pid paths based on the build directory and optional pool name
func GetOrcaPaths(pool string) (string, string) {
	cwd, _ := os.Getwd()
	var buildDir string
	if filepath.Base(cwd) == "build" {
		buildDir = cwd
	} else if _, err := os.Stat(filepath.Join(cwd, "build")); err == nil {
		buildDir = filepath.Join(cwd, "build")
	} else {
		buildDir = filepath.Join(cwd, "build")
	}

	socketName := "flywall-orca.sock"
	pidName := "flywall-orca.pid"
	if pool != "" {
		socketName = fmt.Sprintf("flywall-orca-%s.sock", pool)
		pidName = fmt.Sprintf("flywall-orca-%s.pid", pool)
	}

	return filepath.Join(buildDir, socketName), filepath.Join(buildDir, pidName)
}

// jobState tracks per-job state for log accumulation
type jobState struct {
	Name             string
	Started          bool // True once we've received first output
	StartTime        time.Time
	CurrentStartTime time.Time // For subtest/log context resets
	LogFile          *os.File
	LogPath          string
	Lines            int
	Timeout          time.Duration
	Buffer           []byte                 // Buffer for partial lines
	Skipped          int                    // Count of skipped tests
	Failed           int                    // Count of failed tests
	Total            int                    // Total tests seen
	Todo             bool                   // Test is marked as TODO (allow failure)
	InYaml           bool                   // Are we inside a YAML block?
	YamlBuffer       []string               // Buffer for YAML lines
	Diagnostics      map[string]interface{} // Parsed diagnostics

	// Batch/Subtest tracking
	IsBatch         bool
	CurrentSubtest  string
	SubtestStart    time.Time
	TasksPassed     int
	TasksFailed     int
	TasksSkipped    int
	ExpectedScripts map[string]bool
}

// TestInfo contains the path and timeout for a test
type TestInfo struct {
	Path    string
	Timeout time.Duration
	Scripts []string // For batch jobs: run these scripts in sequence
}

// RunTests submits tests to the orca server and streams results via callback
func RunTests(runID string, tests []TestInfo, logDir string, extraEnv map[string]string, stressDuration time.Duration, pool string, onStart func(string, string), onOutput func(string, []byte), onProgress func(protocol.TestProgress), onResult func(protocol.TestResult)) error {
	socketPath, _ := GetOrcaPaths(pool)
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to orca server: %w (is it running?)", err)
	}
	defer conn.Close()

	runStart := time.Now()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	var wg sync.WaitGroup
	jobs := make(map[string]*jobState) // ID -> state
	var mu sync.Mutex

	// Helper to submit a job
	submitJob := func(t TestInfo) error {
		id := uuid.New().String()

		timeout := t.Timeout
		if timeout == 0 {
			timeout = 90 * time.Second // Default timeout
		}

		env := map[string]string{"TEST_NAME": t.Path}
		if os.Getenv("TEST_DEBUG") != "" {
			env["TEST_DEBUG"] = os.Getenv("TEST_DEBUG")
		}
		// Merge extra environment variables
		for k, v := range extraEnv {
			env[k] = v
		}

		job := protocol.Job{
			ID:         id,
			ScriptPath: t.Path,
			Timeout:    timeout,
			Env:        env,
			Scripts:    t.Scripts,
		}

		req := protocol.ClientRequest{
			Type: "submit_job",
			Job:  job,
		}

		// We need to protect the encoder as it might be called concurrently during stress mode
		// However, we are inside a function. If we call this from the read loop, we have race on encoder.
		// Wait, the read loop is separate goroutine.
		// We need a lock for encoder if we resubmit from read loop.
		// BUT the read loop is NOT calling this directly (it shouldn't).
		// Actually, if we resubmit from "MsgExit" handler (in read loop), we ARE accessing encoder concurrently with initial loop (if overlapping)
		// Or if multiple jobs exit.
		// So we generally need an encoder lock.
		// Let's rely on the fact that initial submission is serial.
		// BUT stress re-submission happens in callback.
		// The caller of this function shouldn't care about internal locks.
		// We'll handle locking here? No, `enc` is local.
		// We need an `encMu` inside RunTests scope.

		// Actually, let's just make `enc` thread-safe by wrapping usage in a mutex.
		// Since we can't change the helper signature easily in this ReplaceBlock without including the mutex definition...
		// I'll define encMu in RunTests scope.

		if err := enc.Encode(req); err != nil {
			return err
		}

		expected := make(map[string]bool)
		for _, s := range t.Scripts {
			expected[s] = true
			expected[filepath.Base(s)] = true
			// Also allow path relative to integration_tests/linux/ or current platform
			rel := s
			if strings.Contains(rel, "integration_tests/") {
				parts := strings.SplitN(rel, "integration_tests/", 2)
				if len(parts) == 2 {
					subParts := strings.SplitN(parts[1], "/", 2)
					if len(subParts) == 2 {
						expected[subParts[1]] = true
					}
				}
			}
		}

		mu.Lock()
		jobs[id] = &jobState{
			Name:             t.Path,
			StartTime:        time.Now(),
			CurrentStartTime: time.Now(),
			Timeout:          timeout,
			IsBatch:          len(t.Scripts) > 0,
			ExpectedScripts:  expected,
		}
		mu.Unlock()
		wg.Add(1)
		return nil
	}

	// We need a mutex for the encoder because stress mode calls submitJob from the reader goroutine
	var encMu sync.Mutex

	// Thread-safe submit wrapper
	safeSubmit := func(t TestInfo) error {
		encMu.Lock()
		defer encMu.Unlock()
		return submitJob(t)
	}

	// Submit all initial jobs
	for _, t := range tests {
		if err := safeSubmit(t); err != nil {
			return err
		}
	}

	// Result loop
	go func() {
		for {
			var msg protocol.Message
			if err := dec.Decode(&msg); err != nil {
				return // Disconnected
			}

			mu.Lock()
			state := jobs[msg.Ref]
			mu.Unlock()

			if state == nil {
				continue
			}

			switch msg.Type {
			case protocol.MsgStdout, protocol.MsgStderr:
				// Create log file on first output (lazy initialization)
				if !state.Started {
					if logDir != "" {
						state.LogFile, state.LogPath, _ = prepLogFile(logDir, state.Name, runID, msg.Ref)
					}
					state.Started = true
					state.StartTime = time.Now()        // Actual start of overall job
					state.CurrentStartTime = time.Now() // For subtest contexts
					if onStart != nil {
						onStart(state.Name, state.LogPath)
					}

					// Inject metadata header
					if state.LogFile != nil && msg.WorkerID != "" {
						fmt.Fprintf(state.LogFile, "### Test Metadata ###\n")
						fmt.Fprintf(state.LogFile, "Test: %s\n", state.Name)
						fmt.Fprintf(state.LogFile, "Worker: %s\n", msg.WorkerID)
						fmt.Fprintf(state.LogFile, "Start: %s\n", state.StartTime.Format(time.RFC3339))

						// Fetch worker history/status
						status, err := GetStatus(pool)
						if err == nil {
							for _, vm := range status.VMs {
								if vm.ID == msg.WorkerID {
									fmt.Fprintf(state.LogFile, "History: %v\n", vm.JobHistory)
									fmt.Fprintf(state.LogFile, "ActiveJobs: %d\n", vm.ActiveJobs)
									break
								}
							}
						}
						fmt.Fprintf(state.LogFile, "---------------------\n\n")
					}

					if onStart != nil {
						onStart(state.Name, state.LogPath)
					}
				}
				if state.LogFile != nil {
					state.LogFile.Write(msg.Data)
				}

				// LINE BUFFERING & TAP PARSING
				// Append new data to buffer
				state.Buffer = append(state.Buffer, msg.Data...)

				// Process complete lines
				for {
					idx := bytes.IndexByte(state.Buffer, '\n')
					if idx == -1 {
						break
					}

					// Extract line (excluding newline)
					line := state.Buffer[:idx]
					// Advance buffer
					state.Buffer = state.Buffer[idx+1:]

					state.Lines++

					lineStr := string(line)
					trimmed := strings.TrimSpace(lineStr)

					if onOutput != nil {
						onOutput(state.Name, line)
					}

					// TAP Parsing Logic
					// Check for Subtest headers first (Batch Mode)
					if strings.HasPrefix(trimmed, "# Subtest:") {
						subtestName := strings.TrimSpace(strings.TrimPrefix(trimmed, "# Subtest:"))
						isScriptSubtest := state.ExpectedScripts[subtestName]

						if state.IsBatch && subtestName != "" && isScriptSubtest {
							// Close previous log if open
							if state.LogFile != nil {
								state.LogFile.Close()
								state.LogFile = nil
							}

							// Start new subtest
							state.CurrentSubtest = subtestName
							state.SubtestStart = time.Now()

							// Open new log file for this subtest
							// LogDir structure: logDir/scriptName.log (flat? or preserve dir?)
							// Let's use prepLogFile logic but with subtest path
							// We need to resolve it relative to something?
							// subtestName is usually the full path on the VM, e.g., "integration_tests/linux/01-sanity/dhcp_test.sh"
							// We want to store it under logDir/integration_tests/linux/01-sanity/dhcp_test.sh.log
							// prepLogFile takes (logDir, testName, runID, jobID).
							// If testName is a path, prepLogFile joins logDir + testName.
							// So if we pass subtestName as testName, it should work.
							f, lPath, err := prepLogFile(logDir, subtestName, runID, msg.Ref)
							if err == nil {
								state.LogFile = f
								state.LogPath = lPath
								state.Started = true
								state.CurrentStartTime = state.SubtestStart // Reset start time for this log context

								// Header
								fmt.Fprintf(state.LogFile, "### Subtest Metadata ###\n")
								fmt.Fprintf(state.LogFile, "Test: %s\n", subtestName)
								fmt.Fprintf(state.LogFile, "Batch: %s\n", state.Name)
								fmt.Fprintf(state.LogFile, "Worker: %s\n", msg.WorkerID)
								fmt.Fprintf(state.LogFile, "Start: %s\n", state.SubtestStart.Format(time.RFC3339))
								fmt.Fprintf(state.LogFile, "---------------------\n\n")

								// No onStart call here; Orca should only track the batch job itself or standalone jobs
							}
						}
					}

					if strings.HasPrefix(trimmed, "ok") {
						state.Total++
						if strings.Contains(lineStr, "# SKIP") {
							state.Skipped++
						}
					} else if strings.HasPrefix(trimmed, "not ok") {
						state.Total++
						// Check for TODO (expected failure)
						// If it's a TODO failure, we don't count it as a failure for the run
						if !strings.Contains(strings.ToLower(lineStr), "# todo") {
							state.Failed++
						}
					}

					if onProgress != nil && (strings.HasPrefix(trimmed, "ok") || strings.HasPrefix(trimmed, "not ok")) {
						onProgress(protocol.TestProgress{
							Name:           state.Name,
							Passed:         state.Total - state.Failed - state.Skipped,
							Failed:         state.Failed,
							Skipped:        state.Skipped,
							Total:          state.Total,
							TasksPassed:    state.TasksPassed,
							TasksFailed:    state.TasksFailed,
							TasksSkipped:   state.TasksSkipped,
							CurrentSubtest: state.CurrentSubtest,
						})
					}

					// Check for Subtest Result (Batch Mode)
					// "ok <N> - <script>" or "not ok <N> - <script>"
					// Only match if we are in batch mode and have a current subtest that is an expected script
					if state.IsBatch && state.CurrentSubtest != "" && state.ExpectedScripts[state.CurrentSubtest] {
						isSubResult := false
						passed := false

						if strings.HasPrefix(trimmed, "ok") && strings.Contains(trimmed, "- "+state.CurrentSubtest) {
							isSubResult = true
							passed = true
						} else if strings.HasPrefix(trimmed, "not ok") && strings.Contains(trimmed, "- "+state.CurrentSubtest) {
							isSubResult = true
							passed = false
						}

						if isSubResult {
							// Emit result for this subtest
							duration := time.Since(state.SubtestStart)

							res := protocol.TestResult{
								ID:       msg.Ref + "-" + state.CurrentSubtest, // Unique-ish ID
								Name:     state.CurrentSubtest,
								Passed:   passed,
								ExitCode: 0, // Assume 0 if passed, but actually command ran.
								Duration: duration,
								LogPath:  state.LogPath,
								WorkerID: msg.WorkerID,
								// We don't have detailed counts for the subtest easily unless we tracked them per subtest
								// For now, let's say Total=1, Failed=0/1 based on outcome.
								Total: 1,
							}

							if !passed {
								res.Failed = 1
								res.ExitCode = 1 // Synthesize exit code
								state.TasksFailed++
							} else {
								state.TasksPassed++
							}
							res.IsSubtest = true

							// No longer suppressing subtests to provide more detail
							if onResult != nil {
								onResult(res)
							}

							// Close log
							if state.LogFile != nil {
								state.LogFile.Close()
								state.LogFile = nil
							}
							state.CurrentSubtest = ""
						}
					}

					// TODO Parsing: Look for "# TODO:" mechanism to allow failure
					if strings.Contains(lineStr, "# TODO:") {
						state.Todo = true
					}

					// YAML Diagnostics Parsing
					// State machine:
					// 0. Default
					// 1. Inside YAML block (saw "  ---")

					if trimmed == "---" {
						state.InYaml = true
						state.YamlBuffer = []string{}
					} else if trimmed == "..." && state.InYaml {
						state.InYaml = false
						// Parse the buffer
						if state.Diagnostics == nil {
							state.Diagnostics = make(map[string]interface{})
						}
						var currentKey string
						var currentBlock []string
						inBlock := false

						for _, yLine := range state.YamlBuffer {
							// Determine indentation
							indent := 0
							for _, r := range yLine {
								if r == ' ' {
									indent++
								} else {
									break
								}
							}

							if inBlock {
								// Our yaml_diag outputs keys at indent 2, block content at indent 4
								if indent > 2 {
									// Block content - strip 4 spaces of indentation if possible
									content := yLine
									if len(content) >= 4 && content[:4] == "    " {
										content = content[4:]
									} else {
										content = strings.TrimSpace(content)
									}
									currentBlock = append(currentBlock, content)
									continue
								} else {
									// End of block
									state.Diagnostics[currentKey] = strings.Join(currentBlock, "\n")
									inBlock = false
								}
							}

							parts := strings.SplitN(yLine, ":", 2)
							if len(parts) == 2 {
								key := strings.TrimSpace(parts[0])
								val := strings.TrimSpace(parts[1])

								if val == "|" {
									currentKey = key
									currentBlock = []string{}
									inBlock = true
								} else {
									state.Diagnostics[key] = strings.Trim(val, "\"")
								}
							}
						}
						// Finalize trailing block
						if inBlock {
							state.Diagnostics[currentKey] = strings.Join(currentBlock, "\n")
						}
					} else if state.InYaml {
						state.YamlBuffer = append(state.YamlBuffer, lineStr)
					}
				}

			case protocol.MsgExit:
				duration := time.Since(state.StartTime)
				passed := msg.ExitCode == 0

				// Check for severity: skip in diagnostics (whole test skipped)
				if val, ok := state.Diagnostics["severity"]; ok {
					if strVal, ok := val.(string); ok && strVal == "skip" {
						state.Skipped = state.Total
					}
				}

				// TAP Check: Fail if we saw any "not ok" lines (unless marked TODO)
				if state.Failed > 0 {
					passed = false
				}

				// Allow failure if marked as TODO
				if state.Todo {
					passed = true
				}

				timedOut := msg.ExitCode == 124 || duration > 85*time.Second

				if state.LogFile != nil {
					state.LogFile.Close()
				}

				result := protocol.TestResult{
					ID:            msg.Ref,
					Name:          state.Name,
					Passed:        passed,
					ExitCode:      msg.ExitCode,
					Duration:      duration,
					LogPath:       state.LogPath,
					TimedOut:      timedOut,
					LinesCaptured: state.Lines,
					WorkerID:      msg.WorkerID,
					Skipped:       state.Skipped,
					Failed:        state.Failed,
					Total:         state.Total,
					TasksPassed:   state.TasksPassed,
					TasksFailed:   state.TasksFailed,
					TasksSkipped:  state.TasksSkipped,
					TasksTotal:    state.TasksPassed + state.TasksFailed + state.TasksSkipped,
					Todo:          state.Todo,
					Diagnostics:   state.Diagnostics,
				}

				// For standalone tests, if they haven't reported Tasks, finalize them as 1 Task
				if !state.IsBatch {
					result.TasksTotal = 1
					if result.Passed {
						result.TasksPassed = 1
					} else {
						result.TasksFailed = 1
					}
				}

				if onResult != nil {
					onResult(result)
				}

				mu.Lock()
				delete(jobs, msg.Ref)
				mu.Unlock()

				// STRESS MODE: Re-submit if duration not expired
				resubmitted := false
				if stressDuration > 0 && time.Since(runStart) < stressDuration {
					// Lookup original test info
					var originalTest TestInfo
					found := false
					for _, t := range tests {
						if t.Path == state.Name {
							originalTest = t
							found = true
							break
						}
					}

					if found {
						// Submit in a goroutine to avoid blocking the reader loop
						// But safeSubmit locks encMu, which is fine.
						// We must increment waitgroup!
						// But wg is local to RunTests.
						// safeSubmit increments wg.
						go func() {
							if err := safeSubmit(originalTest); err != nil {
								fmt.Printf("Error resubmitting stress job: %v\n", err)
							}
							wg.Done()
						}()
						resubmitted = true
					}
				}

				if !resubmitted {
					wg.Done()
				}
			}
		}
	}()

	wg.Wait()
	return nil
}

func prepLogFile(logDir, testName, runID, jobID string) (*os.File, string, error) {
	// Create directory structure: logDir/testName/
	testDir := filepath.Join(logDir, testName)
	if err := os.MkdirAll(testDir, 0755); err != nil {
		return nil, "", err
	}

	// Generate filename from runID and jobID suffix (first 8 chars)
	suffix := jobID
	if len(jobID) > 8 {
		suffix = jobID[:8]
	}
	filename := fmt.Sprintf("%s_%s.log", runID, suffix)
	logPath := filepath.Join(testDir, filename)

	f, err := os.Create(logPath)
	if err != nil {
		return nil, "", err
	}

	// Return absolute or relative path as appropriate
	displayPath := logPath
	// If it's in the project root's build dir, make it a bit nicer?
	// For now just return the path we have.
	return f, displayPath, nil
}

// RunExecWithSocket executes a command on a worker VM using a specific socket
func RunExecWithSocket(command []string, tty bool, vmid string, socketPath string, stdout, stderr io.Writer) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to orca server at %s: %w", socketPath, err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	jobID := uuid.New().String()
	req := protocol.ClientRequest{
		Type:     "exec",
		TargetVM: vmid,
		Command:  command,
		Tty:      tty,
		Job: protocol.Job{
			ID: jobID,
		},
	}
	if command[0] == "/bin/sh" && tty && len(command) == 1 {
		req.Type = "shell" // Semantic helper
	}

	// Lock for Encoder
	var encMu sync.Mutex

	// Helper to send message
	send := func(msg protocol.Message) error {
		encMu.Lock()
		defer encMu.Unlock()
		return enc.Encode(msg)
	}

	// Send initial request
	encMu.Lock()
	if err := enc.Encode(req); err != nil {
		encMu.Unlock()
		return err
	}
	encMu.Unlock()

	// Handle Raw Mode and Resize if interactive TTY
	if tty {
		// Verify terminal and set raw mode
		if term.IsTerminal(os.Stdin.Fd()) {
			oldState, err := term.MakeRaw(os.Stdin.Fd())
			if err == nil {
				defer term.Restore(os.Stdin.Fd(), oldState)
			}
		}

		// Initial Resize
		if w, h, err := term.GetSize(os.Stdin.Fd()); err == nil {
			send(protocol.Message{
				Type:    protocol.MsgResize,
				Ref:     jobID,
				Payload: protocol.ResizePayload{Rows: h, Cols: w},
			})
		}

		// Monitor SIGWINCH
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGWINCH)
		go func() {
			for range sigCh {
				if w, h, err := term.GetSize(os.Stdin.Fd()); err == nil {
					send(protocol.Message{
						Type:    protocol.MsgResize,
						Ref:     jobID,
						Payload: protocol.ResizePayload{Rows: h, Cols: w},
					})
				}
			}
		}()
	}

	// Stdin loop
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			data := make([]byte, n)
			copy(data, buf[:n])
			msg := protocol.Message{
				Type: protocol.MsgStdin,
				Ref:  jobID,
				Data: data,
			}
			send(msg)
		}
	}()

	// Output loop
	for {
		var msg protocol.Message
		if err := dec.Decode(&msg); err != nil {
			return nil // Disconnected
		}

		switch msg.Type {
		case protocol.MsgStdout:
			if stdout != nil {
				stdout.Write(msg.Data)
			}
		case protocol.MsgStderr:
			if stderr != nil {
				stderr.Write(msg.Data)
			}
		case protocol.MsgExit:
			if msg.ExitCode != 0 {
				return fmt.Errorf("exit status %d", msg.ExitCode)
			}
			return nil
		case protocol.MsgError:
			return fmt.Errorf("server error: %s", msg.Error)
		}
	}
}

// RunExec executes a command on a worker VM, potentially with interactivity
func RunExec(command []string, tty bool, vmid string, pool string) error {
	socketPath, _ := GetOrcaPaths(pool)
	return RunExecWithSocket(command, tty, vmid, socketPath, os.Stdout, os.Stderr)
}

// RunShell starts an interactive shell on a worker VM
func RunShell(vmid string, pool string) error {
	return RunExec([]string{"/bin/sh"}, true, vmid, pool)
}

// EnsureServer checks if the orca server is running and starts it if not.
// Returns true if a transient server was started.
func EnsureServer(trace bool, warm, max int, quiet bool, pool string) (bool, error) {
	socketPath, pidPath := GetOrcaPaths(pool)

	// Check if running AND healthy
	if status, err := GetStatus(pool); err == nil {
		// Server running and responsive
		// We could compare status.WarmSize/MaxSize with requested, but for now robustly just return.
		_ = status
		return false, nil
	}

	// Not healthy. Check if dead socket or stale process.
	if !quiet {
		fmt.Println("Orca Server not found or unresponsive.")
	}

	// Read PID file if exists
	if data, err := os.ReadFile(pidPath); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if !quiet {
				fmt.Printf("Cleaning up stale server (PID %d)...\n", pid)
			}
			// Try to kill
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Signal(syscall.SIGTERM)
				// Wait a bit
				time.Sleep(100 * time.Millisecond)
				_ = proc.Kill()
			}
		}
	}

	// Force cleanup paths
	_ = os.Remove(socketPath)
	_ = os.Remove(pidPath)

	if !quiet {
		fmt.Println("Starting transient controller...")
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}

	// Simplified arg logic
	args := []string{"orca", "start"}
	if warm == max {
		args = append(args, fmt.Sprintf("-j%d", max))
	} else {
		args = append(args, fmt.Sprintf("-j%d:%d", warm, max))
	}

	if pool != "" {
		args = append(args, "--pool", pool)
	}

	if trace {
		args = append(args, "--trace")
	}
	cmd := exec.Command(exe, args...)

	// Pass artifact directory environment variable if set
	if artifactDir := os.Getenv("ORCA_ARTIFACT_DIR"); artifactDir != "" {
		cmd.Env = append(os.Environ(), "ORCA_ARTIFACT_DIR="+artifactDir)
	}

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("failed to start transient server: %w", err)
	}

	// Wait for socket
	for i := 0; i < 100; i++ {
		// Verify health, not just connection
		if _, err := GetStatus(pool); err == nil {
			return true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return false, fmt.Errorf("timeout waiting for transient orca server to start")
}

// ShutdownServer sends a shutdown command to the orca server
func ShutdownServer(pool string) error {
	socketPath, _ := GetOrcaPaths(pool)
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	return json.NewEncoder(conn).Encode(protocol.ClientRequest{Type: "shutdown"})
}

// GetStatus fetches the current state of the orca server
func GetStatus(pool string) (*protocol.StatusResponse, error) {
	socketPath, _ := GetOrcaPaths(pool)
	conn, err := net.DialTimeout("unix", socketPath, 1*time.Second) // Add timeout
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Set deadline for request
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	req := protocol.ClientRequest{Type: "status"}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}

	var resp protocol.StatusResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
