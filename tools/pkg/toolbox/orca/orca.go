// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package orca

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/mdlayher/vsock"
	"grimm.is/flywall/tools/pkg/protocol"
	"grimm.is/flywall/tools/pkg/toolbox/harness"
	"grimm.is/flywall/tools/pkg/toolbox/orca/client"
	"grimm.is/flywall/tools/pkg/toolbox/orca/server"
	"grimm.is/flywall/tools/pkg/toolbox/timeouts"
	"grimm.is/flywall/tools/pkg/toolbox/vmm"
)

func printHeader() {
	header := `
  ___ __                        __ __
.'  _|  |.--.--.--.--.--.---.-.|  |  |
|   _|  ||  |  |  |  |  |  _  ||  |  |
|__| |__||___  |________|___._||__|__|
         |_____| test orc(hestr)a(tor)
`
	fmt.Println(header)
}

func Run(args []string) error {
	if len(args) > 0 && (args[0] == "orca" || args[0] == "orchestrator") {
		args = args[1:]
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: orca [run|test]")
	}
	switch args[0] {
	case "run":
		return runVM(args[1:])
	case "demo":
		if len(args) > 1 {
			switch args[1] {
			case "stop":
				return runDemoStop(args[1:])
			case "exec":
				// orca demo exec <cmd...>
				if len(args) < 3 {
					return fmt.Errorf("usage: orca demo exec <command>")
				}
				return runDemoExec(args[2:], false)
			case "shell":
				// orca demo shell
				return runDemoExec(nil, true)
			}
		}
		return runDemo(args[1:])
	case "test":
		return runTests(args[1:])
	case "status":
		return runStatus(args[1:])
	case "shell":
		return runShell(args[1:])
	case "exec":
		return runExec(args[1:])
	case "stop":
		return runStop(args[1:])
	case "history":
		return runHistory(args[1:])
	case "start", "server":
		return runStart(args[1:])
	case "replay":
		return runReplay(args[1:])
	case "tui":
		return runTUI(args[1:])
	case "unit-test":
		return runUnitTest(args[1:])
	case "help", "--help", "-h":
		helpGlobal()
		return nil
	default:
		return fmt.Errorf("unknown command: %s (see 'orca help')", args[0])
	}
}

func helpReplay() {
	fmt.Print(`
Usage: orca replay [RunID]

Replays the output of a previous test run through the TAP parser.
If RunID is not specified, replays the latest run.

Options:
  -h, --help     Show this help message
  --pool [name]  Show status of a specific named pool
`)
}

func helpGlobal() {
	fmt.Println("Flywall Orc(hestr)a(tor) - Integration Test Runner")
	fmt.Println("\nUsage:")
	fmt.Println("  orca <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("\nCommands:")
	fmt.Println("  start     Start the persistent controller daemon")
	fmt.Println("  stop      Stop all running VMs")
	fmt.Println("  demo      Run a live backend demo (interactive, ported)")
	fmt.Println("  test      Run integration tests")
	fmt.Println("  history   View results history and test health matrix")
	fmt.Println("  tui       Interactive test history navigator")
	fmt.Println("  unit-test Run unit tests in a fresh, isolated VM")
	fmt.Println("  run       Start a development VM")
	fmt.Println("  status    Show status of running VMs")
	fmt.Println("  shell     Open a shell in a running VM")
	fmt.Println("  exec      Execute a command in a running VM")
	fmt.Println("  stop      Stop all running VMs")
	fmt.Println("  help      Show this help message")
	fmt.Println("\nUse 'orca <command> --help' for more information on a specific command.")
}

func runStart(args []string) error {
	debug := false
	daemon := true // Default to daemon mode
	trace := false
	warmSize := -1
	maxSize := -1
	pool := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--pool" && i+1 < len(args) {
			pool = args[i+1]
			i++
			continue
		}
		if arg == "-d" || arg == "--debug" || arg == "-v" || arg == "--verbose" {
			debug = true
		}
		if arg == "--foreground" || arg == "-f" {
			daemon = false
		}
		// Support old flag for compatibility (no-op since true is default, unless mixed?)
		if arg == "--daemon" {
			daemon = true
		}

		if arg == "--trace" {
			trace = true
		}
		if arg == "--help" || arg == "-h" {
			helpStart()
			return nil
		}
		if strings.HasPrefix(arg, "-j") {
			jArg := ""
			if len(arg) > 2 {
				jArg = arg[2:]
			} else if i+1 < len(args) {
				jArg = args[i+1]
				i++
			}

			if strings.Contains(jArg, ":") {
				parts := strings.Split(jArg, ":")
				if len(parts) == 2 {
					w, err1 := strconv.Atoi(parts[0])
					m, err2 := strconv.Atoi(parts[1])
					if err1 == nil && err2 == nil && w >= 0 && m > 0 && m >= w {
						warmSize = w
						maxSize = m
						continue
					}
				}
			} else if val, err := strconv.Atoi(jArg); err == nil && val > 0 {
				warmSize = val
				maxSize = val
				continue
			}
		}
	}

	if warmSize == -1 {
		warmSize, maxSize = CalculateOptimalWorkers(!daemon) // Quiet if not daemon? No, start is explicit.
		// Actually runStart is explicit command, so we should probably keep it verbose unless specific flag?
		// But user asked for "orca starts" boilerplate.
		// Let's assume runStart (daemon) is verbose in log, but interactive start might be different.
		// We'll pass false for now as runStart usually goes to log file or user explicitly checking it.
		warmSize, maxSize = CalculateOptimalWorkers(false)
		fmt.Printf("Calculating optimal parallelism... (Workers: %d warm / %d max)\n", warmSize, maxSize)
	}
	if maxSize == -1 {
		maxSize = warmSize
	}

	projectRoot, buildDir := locateBuildDir()

	if daemon {
		logPath := filepath.Join(buildDir, "orca-server.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		exe, err := os.Executable()
		if err != nil {
			return err
		}

		// Re-exec as 'orca start --foreground'
		newArgs := []string{"orca", "start", "--foreground"}
		for _, a := range args {
			// Don't pass --daemon or --foreground again explicitly unless we want to,
			// but we are constructing new command.
			// Just pass everything else?
			// We handle args manually to ensuring clean state
			if a != "--daemon" && a != "--foreground" && a != "-f" {
				newArgs = append(newArgs, a)
			}
		}

		if pool != "" {
			// Ensure --pool is passed to re-exec
			foundPool := false
			for _, a := range newArgs {
				if a == "--pool" {
					foundPool = true
					break
				}
			}
			if !foundPool {
				newArgs = append(newArgs, "--pool", pool)
			}
		}

		cmd := exec.Command(exe, newArgs...)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to daemonize: %w", err)
		}
		fmt.Printf("Orca Server backgrounded (PID %d, Logs: %s)\n", cmd.Process.Pid, logPath)
		os.Exit(0)
	}

	cfg := vmm.Config{
		KernelPath:  filepath.Join(buildDir, "vmlinuz"),
		InitrdPath:  filepath.Join(buildDir, "initramfs"),
		RootfsPath:  filepath.Join(buildDir, "rootfs.qcow2"),
		ProjectRoot: projectRoot,
		Debug:       debug,
		Trace:       trace,
		BuildDir:    buildDir,
	}

	if artifactDir := os.Getenv("ORCA_ARTIFACT_DIR"); artifactDir != "" {
		cfg.ArtifactDir = artifactDir
	}

	socketPath, pidPath := client.GetOrcaPaths(pool)
	srv := server.New(cfg, warmSize, maxSize)
	if err := srv.Start(socketPath, pidPath); err != nil {
		return err
	}

	fmt.Printf("Orca Server listening on %s (Pool: %d warm / %d max)\n", socketPath, warmSize, maxSize)

	// Print artifact directory location if set
	if cfg.ArtifactDir != "" {
		fmt.Printf("Test artifacts will be saved to: %s\n", cfg.ArtifactDir)
	}

	// Start Initial Pool
	for i := 1; i <= warmSize; i++ {
		if err := srv.StartVM(i); err != nil {
			fmt.Printf("Failed to start VM %d: %v\n", i, err)
		}
	}

	// Wait for interruption or remote shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
		fmt.Println("Orca Server shutting down (interrupted)...")
	case <-srv.Done():
		fmt.Println("Orca Server shutting down (command received)...")
	}

	// Ensure we wait for cleanup
	srv.Stop()

	return nil
}

func helpStart() {
	fmt.Println("Usage: orca start [flags]")
	fmt.Println("\nStarts the Orca test orchestration server.")
	fmt.Println("\nFlags:")
	fmt.Println("  -j N              Start with N worker VMs (default: 4)")
	fmt.Println("  -f, --foreground  Run in foreground (default is daemon/background)")
	fmt.Println("  --debug, -d       Show VM console output")
	fmt.Println("  --trace           Log all JSONL protocol messages")
	fmt.Println("  --pool [name]     Store socket/PID in a separate named pool")
	fmt.Println("  --help, -h        Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  orca start -j8")
	fmt.Println("  orca start --foreground --debug")
}

func helpTest() {
	fmt.Println("Usage: orca test [flags] [test_files...]")
	fmt.Println("\nFlags:")
	fmt.Println("  -j N              Run with N transient workers")
	fmt.Println("  -j W:M            Run with W warm workers and M total (M-W transient)")
	fmt.Println("  -filter REGEX     Only run tests matching the regular expression")
	fmt.Println("  -streak-max N     Skip tests with > N consecutive passes (default: 0 = disabled)")
	fmt.Println("  -v                Verbose output (show more details during run)")
	fmt.Println("  --run-skipped     Execute tests marked with SKIP=true")
	fmt.Println("  --only-skipped    Only execute tests marked with SKIP=true")
	fmt.Println("  --strict-isolation Disable worker reuse (fresh VM for every test)")
	fmt.Println("  --cover           Collect Go code coverage from VM binaries")
	fmt.Println("  --trace           Log all JSONL protocol messages")
	fmt.Println("  --pool [name]     Use a specific named pool")
	fmt.Println("  --help, -h        Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  orca test t/01-sanity/*.sh")
	fmt.Println("  orca test -filter dns")
	fmt.Println("  orca test -j8:8")
}

func helpHistory() {
	fmt.Println("Usage: orca history [limit] | [detail <index>]")
	fmt.Println("\nArguments:")
	fmt.Println("  limit           Show summary and health matrix for the last N test runs (default 10)")
	fmt.Println("  detail <index>  Show detailed results for a specific run index (0 is latest)")
	fmt.Println("\nExamples:")
	fmt.Println("  orca history           Show last 10 runs")
	fmt.Println("  orca history 20        Show last 20 runs")
	fmt.Println("  orca history detail 0  Show details for the most recent run")
}

func helpRun() {
	fmt.Println("Usage: orca run")
	fmt.Println("\nStarts a development VM with the Flywall environment configured.")
	fmt.Println("This mode is used for interactive testing and feature development.")
}

func helpStatus() {
	fmt.Println("Usage: orca status [--pool name]")
	fmt.Println("\nScans and displays the status of all currently active VMs managed by Orca.")
	fmt.Println("Use --pool to check a specific named pool.")
	fmt.Println("In Linux, this uses vsock; in macOS/others, it uses Unix domain sockets.")
}

func helpShell() {
	fmt.Println("Usage: orca shell [options]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --vmid, -v <id>   Target a specific VM by ID")
	fmt.Println("  --pool <name>     Target a specific named pool")
	fmt.Println("  --help, -h        Show this help message")
	fmt.Println("\nConnects to an active VM and opens an interactive shell session.")
	fmt.Println("If no VM is running, it will automatically bootstrap a temporary session.")
}

func helpExec() {
	fmt.Println("Usage: orca exec [options] -- <command>")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --vmid, -v <id>   Target a specific VM by ID")
	fmt.Println("  --pool <name>     Target a specific named pool")
	fmt.Println("  --help, -h        Show this help message")
	fmt.Println("\nExecutes a non-interactive command inside a VM.")
	fmt.Println("If no VM is running, it will automatically bootstrap a temporary session.")
	fmt.Println("\nArguments:")
	fmt.Println("  command    The command to execute in the VM")
	fmt.Println("\nExample:")
	fmt.Println("  orca exec ip addr show")
}

func helpStop() {
	fmt.Println("Usage: orca stop [--pool name]")
	fmt.Println("\nSends a shutdown signal to the warm worker pool and cleans up active sessions.")
}

// locateBuildDir returns the project root and the build directory.
// It handles running from both project root and the build directory itself.
func locateBuildDir() (string, string) {
	cwd, _ := os.Getwd()
	// If the current directory is named "build", assume we are inside it.
	if filepath.Base(cwd) == "build" {
		return filepath.Dir(cwd), cwd
	}
	// If a "build" subdirectory exists, assume we are in the project root.
	if _, err := os.Stat(filepath.Join(cwd, "build")); err == nil {
		return cwd, filepath.Join(cwd, "build")
	}
	// Fallback
	return cwd, filepath.Join(cwd, "build")
}

func runHistory(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		helpHistory()
		return nil
	}
	_, buildDir := locateBuildDir() // projectRoot unused

	history, err := LoadHistory(buildDir)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	if len(args) > 0 && args[0] == "detail" {
		if len(args) > 1 {
			if idx, err := strconv.Atoi(args[1]); err == nil {
				history.PrintDetail(idx)
			} else {
				fmt.Println("Invalid index format. Usage: orca history detail <index>")
			}
		} else {
			history.PrintDetail(0) // Default to latest
		}
		return nil
	}

	limit := 10
	if len(args) > 0 {
		if l, err := strconv.Atoi(args[0]); err == nil {
			limit = l
		}
	}

	history.PrintSummary(limit, nil)
	return nil
}

func runVM(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		helpRun()
		return nil
	}
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	if isTerminal {
		printHeader()
	} else {
		fmt.Println("Flywall Orc(hestr)a(tor) Starting...")
	}
	// Defaults based on scripts/vm-dev.sh
	projectRoot, buildDir := locateBuildDir()

	cfg := vmm.Config{
		KernelPath:  filepath.Join(buildDir, "vmlinuz"),
		InitrdPath:  filepath.Join(buildDir, "initramfs"),
		RootfsPath:  filepath.Join(buildDir, "rootfs.qcow2"),
		ProjectRoot: projectRoot,
		Debug:       true,
		BuildDir:    buildDir,
	}

	vm, err := vmm.NewVM(cfg, 1) // ID 1 for single VM mode
	if err != nil {
		return err
	}
	defer vm.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived signal, stopping VM...")
		cancel()
	}()

	if err := vm.Start(ctx); err != nil {
		return fmt.Errorf("vm error: %w", err)
	}

	return nil
}

func runTests(args []string) error {
	// Defaults
	projectRoot, buildDir := locateBuildDir()
	cwd := projectRoot

	// Load test history early for test discovery
	history, err := LoadHistory(buildDir)
	if err != nil {
		fmt.Printf("Warning: failed to load test history: %v\n", err)
		history = &TestHistory{MaxRuns: DefaultMaxRuns}
	}

	// Start resource monitoring
	go monitorResources(cwd)

	// Parse args
	warmSize := -1 // Default warm pool size
	maxSize := -1  // 0 means no overflow (maxSize = warmSize)
	runSkipped := false
	onlySkipped := false
	verbose := false
	streakMax := 0
	strictIsolation := false
	trace := false
	noShuffle := false
	rerunFailed := false
	failedRuns := 1 // Default to checking last 1 run
	checkModified := false
	var filter string
	var target string
	var tests []TestJob
	cover := false
	stressMode := "standard"
	stressDuration := time.Duration(0)
	pool := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--pool" && i+1 < len(args) {
			pool = args[i+1]
			i++
			continue
		}
		if arg == "--help" || arg == "-h" || arg == "help" {
			helpTest()
			return nil
		}
		if arg == "--mode" || strings.HasPrefix(arg, "--mode=") {
			// Handle --mode=stress or --mode stress
			val := ""
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				val = parts[1]
			} else if i+1 < len(args) {
				val = args[i+1]
				i++
			}
			if val != "stress" && val != "standard" {
				return fmt.Errorf("invalid mode: %s (allowed: standard, stress)", val)
			}
			stressMode = val
			continue
		}
		if arg == "--duration" || strings.HasPrefix(arg, "--duration=") {
			val := ""
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				val = parts[1]
			} else if i+1 < len(args) {
				val = args[i+1]
				i++
			}
			d, err := time.ParseDuration(val)
			if err != nil {
				return fmt.Errorf("invalid duration: %v", err)
			}
			stressDuration = d
			continue
		}
		if arg == "--rerun-failed" || arg == "--failed" {
			rerunFailed = true
			failedRuns = 1
			continue
		}
		if strings.HasPrefix(arg, "--failed=") {
			rerunFailed = true
			val := strings.TrimPrefix(arg, "--failed=")
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				failedRuns = n
			} else {
				return fmt.Errorf("invalid --failed value: %s (must be a positive integer)", val)
			}
			continue
		}
		if arg == "--changed" || arg == "--modified" {
			checkModified = true
			continue
		}
		if arg == "--run-skipped" {
			runSkipped = true
			continue
		}
		if arg == "--only-skipped" {
			onlySkipped = true
			runSkipped = true // implies running skipped tests
			continue
		}
		if strings.HasPrefix(arg, "-j") {
			jArg := ""
			if len(arg) > 2 {
				jArg = arg[2:]
			} else if i+1 < len(args) {
				jArg = args[i+1]
				i++
			} else {
				return fmt.Errorf("missing -j value")
			}

			// Check for warm:max format
			if strings.Contains(jArg, ":") {
				parts := strings.Split(jArg, ":")
				if len(parts) == 2 {
					w, err1 := strconv.Atoi(parts[0])
					m, err2 := strconv.Atoi(parts[1])
					if err1 == nil && err2 == nil && w >= 0 && m > 0 && m >= w {
						warmSize = w
						maxSize = m
						continue
					}
				}
				return fmt.Errorf("invalid -j format, use: -j N or -j warm:max")
			}
			// Simple number format - all transient
			val, err := strconv.Atoi(jArg)
			if err == nil && val > 0 {
				warmSize = 0  // No warm workers
				maxSize = val // All transient
				continue
			}
			return fmt.Errorf("invalid -j value: %s", jArg)
		}

		if arg == "-filter" {
			if i+1 < len(args) {
				filter = args[i+1]
				i++
				continue
			}
			return fmt.Errorf("missing value for -filter")
		}

		if arg == "-v" {
			verbose = true
			continue
		}

		if arg == "-streak-max" {
			if i+1 < len(args) {
				val, err := strconv.Atoi(args[i+1])
				if err == nil && val > 0 {
					streakMax = val
					i++
					continue
				}
			}
			return fmt.Errorf("invalid or missing value for -streak-max")
		}

		if arg == "--strict-isolation" {
			strictIsolation = true
			continue
		}

		if arg == "--trace" {
			trace = true
			continue
		}

		if arg == "--cover" {
			cover = true
			continue
		}

		if arg == "--no-shuffle" {
			// Disable shuffling (for deterministic debugging)
			// We can't easily propagate this via a variable due to how I structured parsing,
			// so just setting a special env var or using a variable in scope (which IS in scope here).
			// Wait, I need 'noShuffle' defined.
			continue
		}

		if arg == "--no-shuffle" {
			noShuffle = true
			continue
		}

		if arg == "--target" {
			if i+1 < len(args) {
				target = args[i+1]
				i++
				continue
			}
			return fmt.Errorf("missing value for --target")
		}

		// Check if it's a wildcard pattern
		if strings.Contains(arg, "*") {
			matches, err := filepath.Glob(arg)
			if err == nil && len(matches) > 0 {
				for _, m := range matches {
					if info, err := os.Stat(m); err == nil && !info.IsDir() {
						timeout := parseTestTimeout(m)
						tests = append(tests, TestJob{ScriptPath: m, Timeout: timeout})
					}
				}
				continue
			}
			// If glob failed or no matches, try relative to integration root if target set
			if target != "" && !filepath.IsAbs(arg) {
				candidate := filepath.Join(projectRoot, "integration_tests", target, arg)
				matches, err := filepath.Glob(candidate)
				if err == nil && len(matches) > 0 {
					for _, m := range matches {
						if info, err := os.Stat(m); err == nil && !info.IsDir() {
							timeout := parseTestTimeout(m)
							tests = append(tests, TestJob{ScriptPath: m, Timeout: timeout})
						}
					}
					continue
				}
			}
		}

		// Check if it's a file or directory
		pathToCheck := arg
		if target != "" && !filepath.IsAbs(arg) {
			// Try checking if it exists under integration_tests/<target>/arg
			// e.g. target=linux, arg=10-api/api_test.sh -> integration_tests/linux/10-api/api_test.sh
			candidate := filepath.Join(projectRoot, "integration_tests", target, arg)
			if _, err := os.Stat(candidate); err == nil {
				pathToCheck = candidate
			}
		}

		if info, statErr := os.Stat(pathToCheck); statErr == nil {
			if info.IsDir() {
				// First pass: find all batch directories
				batchDirs := make(map[string]bool)
				filepath.Walk(pathToCheck, func(path string, fInfo os.FileInfo, err error) error {
					if err != nil || fInfo.IsDir() {
						return nil
					}
					if fInfo.Name() == "BATCH" {
						batchDirs[filepath.Dir(path)] = true
					}
					return nil
				})

				// Second pass: collect tests, grouping batch directories
				batchScripts := make(map[string][]string)
				batchTimeouts := make(map[string]time.Duration)

				filepath.Walk(pathToCheck, func(path string, fInfo os.FileInfo, err error) error {
					if err != nil || fInfo.IsDir() {
						return nil
					}
					if strings.HasSuffix(path, "_test.sh") {
						dir := filepath.Dir(path)
						if batchDirs[dir] {
							// Add to batch group
							batchScripts[dir] = append(batchScripts[dir], path)
							batchTimeouts[dir] += parseTestTimeout(path)
						} else {
							// Individual test
							timeout := parseTestTimeout(path)
							tests = append(tests, TestJob{ScriptPath: path, Timeout: timeout})
						}
					}
					return nil
				})

				// Create batch jobs
				for dir, scripts := range batchScripts {
					// Sort for deterministic order
					for i := 0; i < len(scripts)-1; i++ {
						for j := i + 1; j < len(scripts); j++ {
							if scripts[i] > scripts[j] {
								scripts[i], scripts[j] = scripts[j], scripts[i]
							}
						}
					}

					relDir := filepath.Clean(dir)
					tests = append(tests, TestJob{
						ScriptPath: relDir + "/*",
						BatchDir:   relDir,
						Scripts:    scripts,
						Timeout:    batchTimeouts[dir] + 30*time.Second,
					})
				}
			} else {
				// Single file
				timeout := parseTestTimeout(pathToCheck)
				tests = append(tests, TestJob{ScriptPath: pathToCheck, Timeout: timeout})
			}
		} else {
			// Try fuzzy/substring matching
			allTests, err := DiscoverTests(projectRoot, target, nil)
			if err == nil {
				var matched []TestJob
				for _, t := range allTests {
					if strings.Contains(t.ScriptPath, arg) || strings.Contains(filepath.Base(t.ScriptPath), arg) {
						matched = append(matched, t)
					}
				}
				if len(matched) > 0 {
					tests = append(tests, matched...)
				} else {
					fmt.Printf("Warning: No tests found matching '%s'\n", arg)
				}
			} else {
				fmt.Printf("Warning: Failed to discover tests for fuzzy matching: %v\n", err)
				fmt.Printf("Warning: Test file not found: %s\n", arg)
			}
		}
	}

	if maxSize == 0 {
		maxSize = 4 // Fallback if something went wrong
	}

	// Check TTY for startup banner
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

	if isTerminal {
		printHeader()
	} else {
		fmt.Println("Flywall Orc(hestr)a(tor) Starting...")
	}

	// Ensure sane defaults
	if warmSize == -1 {
		// Heuristics
		warmSize, maxSize = CalculateOptimalWorkers(isTerminal)
		if !isTerminal {
			fmt.Printf("Calculating optimal parallelism... (Workers: %d warm / %d max)\n", warmSize, maxSize)
		}
	}

	if maxSize == -1 || maxSize == 0 {
		maxSize = warmSize
	}
	if maxSize == 0 {
		maxSize = 4 // Fallback if something went wrong
	}

	// Propagate verbosity to config
	/* cfg := vmm.Config{
		KernelPath:    filepath.Join(buildDir, "vmlinuz"),
		InitrdPath:    filepath.Join(buildDir, "initramfs"),
		RootfsPath:    filepath.Join(buildDir, "rootfs.qcow2"),
		ProjectRoot:   cwd,
		RunSkipped:    runSkipped,
		Verbose:       verbose,
		StrictIsolation: strictIsolation,
	} */

	// Handle --rerun-failed
	if rerunFailed {
		if history == nil || len(history.Tests) == 0 {
			return fmt.Errorf("no test history found to rerun failures from")
		}

		if failedRuns == 1 {
			fmt.Println("Searching for tests whose most recent run was not successful...")
		} else {
			fmt.Printf("Searching for tests with failures in their last %d runs...\n", failedRuns)
		}
		// Get the latest run ID
		if len(history.RunMeta) == 0 {
			return fmt.Errorf("no previous runs found in history")
		}
		lastRunID := history.RunMeta[len(history.RunMeta)-1].RunID

		// Find tests that have failed in their last N runs
		var failedTests []string
		count := 0
		for path, stats := range history.Tests {
			if len(stats.Executions) > 0 {
				// Sort executions by timestamp descending
				executions := make([]TestExecution, len(stats.Executions))
				copy(executions, stats.Executions)
				sort.Slice(executions, func(i, j int) bool {
					return executions[i].Timestamp.After(executions[j].Timestamp)
				})

				// Take the last N executions (or all if fewer)
				n := failedRuns
				if len(executions) < n {
					n = len(executions)
				}

				// Check if any of the last N executions were not successful
				hasFailure := false
				for i := 0; i < n; i++ {
					if executions[i].Status != "pass" && executions[i].Status != "skip" {
						hasFailure = true
						break
					}
				}

				if hasFailure {
					failedTests = append(failedTests, path)
					count++
				}
			}
		}

		if count == 0 {
			if failedRuns == 1 {
				fmt.Println("No failed tests found (all tests passed on their most recent run)!")
			} else {
				fmt.Printf("No failed tests found (all tests passed in their last %d runs)!\n", failedRuns)
			}
			return nil
		}

		fmt.Printf("Queuing %d failed tests from run %s\n", count, lastRunID)

		// Convert to TestJob
		// We need to resolve full paths if history stores relative ones...
		// History stores whatever ScriptPath was. Usually relative to projectRoot.
		for _, p := range failedTests {
			// Find timeout if possible (re-parse or default)
			fullPath := filepath.Join(projectRoot, p)
			if _, err := os.Stat(fullPath); err != nil {
				// Try removing project root if it was absolute
				// Or check if p is already correct
				if _, err2 := os.Stat(p); err2 == nil {
					fullPath = p
				}
			}

			timeout := parseTestTimeout(fullPath)
			tests = append(tests, TestJob{ScriptPath: p, Timeout: timeout})
		}
	} else if checkModified {
		// Handle --changed/--modified
		if history == nil || len(history.Tests) == 0 {
			return fmt.Errorf("no test history found to check modified tests")
		}

		fmt.Println("Searching for tests modified since their last passing run...")

		var modifiedTests []string
		count := 0

		for path, stats := range history.Tests {
			if len(stats.Executions) == 0 {
				continue
			}

			// Find the last passing run
			var lastPassTime time.Time
			for _, exec := range stats.Executions {
				if exec.Status == "pass" && exec.Timestamp.After(lastPassTime) {
					lastPassTime = exec.Timestamp
				}
			}

			if !lastPassTime.IsZero() {
				// Check file modification time
				fullPath := filepath.Join(projectRoot, path)
				if _, err := os.Stat(fullPath); err != nil {
					// Try alternative path
					fullPath = path
					if _, err2 := os.Stat(fullPath); err2 != nil {
						continue // File doesn't exist
					}
				}

				fileInfo, err := os.Stat(fullPath)
				if err != nil {
					continue
				}

				// If file was modified after last pass, add it
				if fileInfo.ModTime().After(lastPassTime) {
					modifiedTests = append(modifiedTests, path)
					count++
				}
			}
		}

		if count == 0 {
			fmt.Println("No modified tests found (all tests are up to date)!")
			return nil
		}

		fmt.Printf("Found %d tests modified since their last passing run\n", count)

		// Convert to TestJob
		for _, p := range modifiedTests {
			fullPath := filepath.Join(projectRoot, p)
			if _, err := os.Stat(fullPath); err != nil {
				if _, err2 := os.Stat(p); err2 == nil {
					fullPath = p
				} else {
					continue
				}
			}

			timeout := parseTestTimeout(fullPath)
			tests = append(tests, TestJob{ScriptPath: p, Timeout: timeout})
		}
	} else if len(tests) == 0 {
		tests, err = DiscoverTests(projectRoot, target, history)
		if err != nil {
			return fmt.Errorf("failed to discover tests: %w", err)
		}
	}

	// Suppress unused variable errors for flags not yet re-implemented in V2 Client
	_ = verbose
	_ = strictIsolation
	_ = maxSize
	_ = trace
	_ = runSkipped
	_ = onlySkipped
	_ = streakMax

	// Apply filter if specified
	if filter != "" {
		re, err := regexp.Compile(filter)
		if err != nil {
			return fmt.Errorf("invalid filter regex: %w", err)
		}
		var filtered []TestJob
		for _, t := range tests {
			if re.MatchString(t.ScriptPath) {
				filtered = append(filtered, t)
			}
		}
		tests = filtered
	}

	// Apply skip filtering
	var skipCount int
	var skippedTests []string

	if streakMax > 0 {
		// Filter by success streak
		history, err := LoadHistory(buildDir)
		if err == nil { // silently ignore history load errors here
			var streakFiltered []TestJob
			for _, t := range tests {
				streak := history.GetStreak(t.ScriptPath)
				if streak <= streakMax {
					streakFiltered = append(streakFiltered, t)
				} else {
					skipCount++
					skippedTests = append(skippedTests, fmt.Sprintf("%s (streak: %d)", t.ScriptPath, streak))
				}
			}
			tests = streakFiltered
		}
	}

	if onlySkipped {
		// Only run skipped tests
		var skippedOnly []TestJob
		for _, t := range tests {
			if t.Skip {
				skippedOnly = append(skippedOnly, t)
			}
		}
		skipCount = len(tests) - len(skippedOnly)
		tests = skippedOnly
	} else if !runSkipped {
		// Exclude skipped tests (default behavior)
		var notSkipped []TestJob
		for _, t := range tests {
			if !t.Skip {
				notSkipped = append(notSkipped, t)
			} else {
				skipCount++
				skippedTests = append(skippedTests, t.ScriptPath)
			}
		}
		tests = notSkipped

		// Display skipped tests
		if len(skippedTests) > 0 {
			fmt.Printf("Skipping %d test(s):\n", len(skippedTests))
			for _, path := range skippedTests {
				fmt.Printf("  🚧  %s\n", path)
			}
		}
	}

	// Shuffle test order to identify ordering-dependent tests
	if !noShuffle {
		rand.Shuffle(len(tests), func(i, j int) {
			tests[i], tests[j] = tests[j], tests[i]
		})
	}

	if len(tests) == 0 {
		if skipCount > 0 {
			fmt.Printf("No tests found (%d skipped)\n", skipCount)
		} else {
			fmt.Println("No tests found")
		}
		return nil
	}

	// 1. Calculate script counts and display mappings early for accurate reporting
	var testScripts []string
	for _, t := range tests {
		if len(t.Scripts) > 0 {
			testScripts = append(testScripts, t.Scripts...)
		} else {
			testScripts = append(testScripts, t.ScriptPath)
		}
	}

	// Calculate if some tests were skipped (usually at job level)
	skipMsg := ""
	if skipCount > 0 && !runSkipped {
		// Currently skipCount tracks jobs, but for consistency we should ideally
		// track scripts if we are reporting script counts.
		// However, the jobs in 'tests' are what's being RUN.
		// For now, keep skipCount as is but ensure the primary 'Found' message is script-based.
		skipMsg = fmt.Sprintf(" (skipping %d)", skipCount)
	}

	// Determine optimal reporting workers
	// Optimization: If transient runners requested (warmSize=0) and not connecting to existing pool,
	// cap maxSize to number of tests to avoid spinning up more VMs than needed.
	if warmSize == 0 && len(tests) < maxSize {
		maxSize = len(tests)
	}

	reportWarm := warmSize
	reportMax := maxSize

	// Generate Run ID for this test session
	startTime := time.Now()
	runID := startTime.Format("20060102-150405") + "_" + fmt.Sprintf("%08x", rand.Uint32())

	// Create persistent artifact directory
	artifactDir := filepath.Join(buildDir, "test-artifacts", runID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}

	// Set environment variable for server to use (must be before EnsureServer)
	os.Setenv("ORCA_ARTIFACT_DIR", artifactDir)

	if !isTerminal {
		fmt.Println("Ensuring Orca Server is running...")
	} else {
		// Dynamic status line
		fmt.Print("\rEnsuring Orca Server is running... ")
	}

	transient, err := client.EnsureServer(trace, warmSize, maxSize, isTerminal, pool)
	if err != nil {
		return err
	}

	if isTerminal {
		fmt.Print("\r\033[K") // Clear line
	}

	// Fetch ACTUAL server status to report correctly
	status, err := client.GetStatus(pool)
	if err == nil {
		reportWarm = status.WarmSize
		reportMax = status.MaxSize
	}

	if !isTerminal {
		if reportWarm > 0 {
			fmt.Printf("Found %d tests%s, running with %d warm + %d overflow workers\n",
				len(testScripts), skipMsg, reportWarm, reportMax-reportWarm)
		} else {
			fmt.Printf("Found %d tests%s, running with up to %d transient workers\n",
				len(testScripts), skipMsg, reportMax)
		}
	}

	if !isTerminal {
		if target != "" {
			fmt.Printf("Platform Target: %s (integration_tests/%s)\n", target, target)
		} else {
			fmt.Printf("Platform Target: default (integration_tests/linux)\n")
		}

		// Log TIME_DILATION factor for visibility
		factor := timeouts.GetFactor()
		fmt.Printf("TIME_DILATION: %.2fx (test timeouts scaled)\n", factor)

		fmt.Printf("Run ID: %s\n", runID)
	} else {
		// Compact Summary
		tgt := "default"
		if target != "" {
			tgt = target
		}
		fmt.Printf("🚀 %d Tests | Target: %s | Workers: %d/%d | RunID: %s\n", len(testScripts), tgt, reportWarm, reportMax, runID)
	}

	fmt.Println()
	fmt.Printf("%-2s %-45s [ %s | %s | %s ] %s\n", "🏁", "TEST NAME", "✅", "❌", "🚧", "DURATION")
	fmt.Println(strings.Repeat("─", 85))

	// Create test results base directory
	resultsBase := filepath.Join(cwd, "build", "test-results")
	if err := os.MkdirAll(resultsBase, 0755); err != nil {
		return fmt.Errorf("failed to create results dir: %w", err)
	}

	// Extract test info with timeouts
	var testInfos []client.TestInfo
	for _, t := range tests {
		testInfos = append(testInfos, client.TestInfo{
			Path:    t.ScriptPath,
			Timeout: t.Timeout,
			Scripts: t.Scripts,
		})
	}

	displayMap := computeDisplayNames(testScripts)

	if transient {
		defer func() {
			fmt.Println("Shutting down transient controller...")
			client.ShutdownServer(pool)
		}()
	}

	// Track results for summary
	var results []protocol.TestResult
	var resultsMu sync.Mutex
	completed := 0
	total := len(testScripts)

	// Track which tests have completed
	completedTests := make(map[string]bool)
	testLogs := make(map[string]string) // Name -> LogPath

	// Stats for status bar
	passedCount := 0
	failedCount := 0
	skippedCount := 0

	// Remove redeclaration of isTerminal
	// isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

	type runState struct {
		start                                  time.Time
		passed, failed, skipped                int    // Assertions
		tasksPassed, tasksFailed, tasksSkipped int    // Scripts/Tasks
		currentSubtest                         string // Active sub-script
	}
	runningTests := make(map[string]*runState)
	footerHeight := 0 // total lines including status

	printStatus := func() {
		if !isTerminal {
			return
		}

		resultsMu.Lock()
		curPassed := passedCount
		curFailed := failedCount
		curSkipped := skippedCount
		curCompleted := completed

		for _, s := range runningTests {
			curPassed += s.tasksPassed
			curFailed += s.tasksFailed
			curSkipped += s.tasksSkipped
			curCompleted += (s.tasksPassed + s.tasksFailed + s.tasksSkipped)
		}

		// Sort running tests by start time for stability
		type runInfo struct {
			name  string
			state *runState
		}
		var sorted []runInfo
		for n, s := range runningTests {
			sorted = append(sorted, runInfo{n, s})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].state.start.Before(sorted[j].state.start)
		})
		resultsMu.Unlock()

		newFooterHeight := 1 // status line
		if !verbose {
			for _, ri := range sorted {
				dur := time.Since(ri.state.start)
				mins := int(dur.Minutes())
				secs := int(dur.Seconds()) % 60
				durStr := fmt.Sprintf("%d:%02d", mins, secs)

				displayName := formatDisplayName(ri.name, ri.state.currentSubtest, target)
				lineStr := fmt.Sprintf("%-2s %s [%3d |%3d |%3d ] %-9s    ", "⏳", padRight(displayName, 45), ri.state.passed, ri.state.failed, ri.state.skipped, durStr)
				newFooterHeight += getVisualLineCount(lineStr)
			}
		}

		// Add wrapped count for status line
		totalDur := time.Since(startTime).Round(time.Millisecond)
		mins := int(totalDur.Minutes())
		secs := totalDur.Seconds() - float64(mins*60)
		durStr := fmt.Sprintf("%02d:%05.2f", mins, secs)
		statusStr := fmt.Sprintf("Status: %d/%d completed (✅ %d, ❌ %d, 🚧 %d) - %s", curCompleted, total, curPassed, curFailed, curSkipped, durStr)
		newFooterHeight += getVisualLineCount(statusStr) - 1 // -1 because it doesn't end in \n yet

		// Move up to the existing footer's top
		if footerHeight > 1 {
			fmt.Printf("\033[%dA", footerHeight-1)
		}
		fmt.Print("\r\033[J") // Clear from current position to bottom

		if !verbose {
			for _, ri := range sorted {
				dur := time.Since(ri.state.start)
				mins := int(dur.Minutes())
				secs := int(dur.Seconds()) % 60
				durStr := fmt.Sprintf("%d:%02d", mins, secs)

				displayName := formatDisplayName(ri.name, ri.state.currentSubtest, target)
				// Manual padding with visual width awareness, left-aligned duration
				lineStr := fmt.Sprintf("%-2s %s [%3d |%3d |%3d ] %-9s    ", "⏳", padRight(displayName, 45), ri.state.passed, ri.state.failed, ri.state.skipped, durStr)
				fmt.Println(lineStr)
			}
		}

		totalDur = time.Since(startTime).Round(time.Millisecond)
		// Format total duration
		mins = int(totalDur.Minutes())
		secs = totalDur.Seconds() - float64(mins*60)
		durStr = fmt.Sprintf("%d:%05.2f", mins, secs)
		fmt.Printf("Status: %d/%d completed (✅ %d, ❌ %d, 🚧 %d) - %s", curCompleted, total, curPassed, curFailed, curSkipped, durStr)

		footerHeight = newFooterHeight
	}

	logDir := filepath.Join(cwd, "build", "test-results")

	onStart := func(name, path string) {
		resultsMu.Lock()
		testLogs[name] = path
		runningTests[name] = &runState{start: time.Now()}
		resultsMu.Unlock()
		if isTerminal && !verbose {
			printStatus()
		}
	}

	onProgress := func(p protocol.TestProgress) {
		resultsMu.Lock()
		if s, ok := runningTests[p.Name]; ok {
			s.passed = p.Passed
			s.failed = p.Failed
			s.skipped = p.Skipped
			s.tasksPassed = p.TasksPassed
			s.tasksFailed = p.TasksFailed
			s.tasksSkipped = p.TasksSkipped
			s.currentSubtest = p.CurrentSubtest
		}
		resultsMu.Unlock()
		// No need to repaint here if ticker is running, but let's do it for responsiveness
		if isTerminal && !verbose {
			printStatus()
		}
	}

	onOutput := func(name string, line []byte) {
		if verbose {
			// Clear footer
			if isTerminal {
				if footerHeight > 1 {
					fmt.Printf("\033[%dA", footerHeight-1)
				}
				fmt.Print("\r\033[J")
				footerHeight = 0
			}
			displayName := displayMap[name]
			if displayName == "" {
				displayName = name
			}
			fmt.Printf("[%s] %s\n", displayName, harness.HighlightTAP(string(line)))
			// Repaint footer
			printStatus()
		}
	}

	onResult := func(r protocol.TestResult) {
		resultsMu.Lock()
		results = append(results, r)
		completedTests[r.Name] = true
		delete(runningTests, r.Name)
		resultsMu.Unlock()

		// Update stats
		failedCount += r.TasksFailed
		skippedCount += r.TasksSkipped
		passedCount += r.TasksPassed
		completed += r.TasksTotal

		// Clear footer before printing permanent result
		if isTerminal {
			if footerHeight > 1 {
				fmt.Printf("\033[%dA", footerHeight-1)
			}
			fmt.Print("\r\033[J")
			footerHeight = 0
		}

		// Format duration as MM:SS.mmm
		dur := r.Duration
		mins := int(dur.Minutes())
		secs := dur.Seconds() - float64(mins*60)
		durStr := fmt.Sprintf("%d:%06.3f", mins, secs)

		// Display result
		marker := "✅"
		extra := ""

		// Calculate stats integers
		passInt := 0
		failInt := r.Failed
		skipInt := r.Skipped

		if r.Total > 0 {
			realPass := r.Total - r.Failed - r.Skipped
			if realPass < 0 {
				realPass = 0
			} // Safety
			passInt = realPass
		}

		if !r.Passed {
			marker = "❌"
			if r.TimedOut {
				extra = " ⏱"
			}
		} else if r.Total > 0 && r.Skipped == r.Total {
			marker = "🚧 "
			extra += fmt.Sprintf(" (skipped %d/%d)", r.Skipped, r.Total)
		} else if r.Todo {
			marker = "📝"
			extra = " (TODO)"
			if r.ExitCode == 0 {
				extra += " (Unexpected Pass)"
			}
			if r.Skipped > 0 {
				extra += fmt.Sprintf(" (skipped %d)", r.Skipped)
			}
		} else {
			// Normal pass
			if isAnom, _, expected := history.IsAnomalous(r.Name, r.Duration); isAnom && expected > 0 {
				pct := float64(r.Duration-expected) / float64(expected) * 100
				if pct > 0 {
					extra += fmt.Sprintf(" 🐢 +%.0f%%", pct)
				} else {
					extra += fmt.Sprintf(" 🐇 %.0f%%", pct)
				}
			}
		}

		// Relativize and format path
		displayName := formatDisplayName(r.Name, "", target)

		if strings.HasSuffix(r.Name, "/*") {
			return // Omit the batch envelope from the final log
		}

		// Manual padding for visual consistency
		fmt.Printf("%-2s %s [%3d |%3d |%3d ] %-9s%s\n", marker, padRight(displayName, 45), passInt, failInt, skipInt, durStr, extra)

		if !r.Passed && r.TimedOut {
			fmt.Printf("   └─ test exceeded timeout (captured %d lines before failure)\n", r.LinesCaptured)
		}

		if len(r.Diagnostics) > 0 {
			for k, v := range r.Diagnostics {
				// Suppress "severity: skip" if we already show it as skipped
				if k == "severity" && fmt.Sprint(v) == "skip" && r.Total > 0 && r.Skipped == r.Total {
					continue
				}
				// nice formatting
				fmt.Printf("   └─ %s: %v\n", k, v)
			}
		}

		// Repaint status line
		printStatus()
	}

	saveHistory := func(res []protocol.TestResult) {
		// Transform Results to []WorkerRun (flattened for now since we just list all results)
		// We grouping by WorkerID to fit the history struct
		workerRuns := make(map[string]*WorkerRun)
		finalPassed := 0
		finalFailed := 0
		finalSkipped := 0

		for _, r := range res {
			if r.IsSubtest {
				continue // Already accounted for in envelope
			}

			finalPassed += r.TasksPassed
			finalFailed += r.TasksFailed
			finalSkipped += r.TasksSkipped

			wid := r.WorkerID
			wIDInt := 0
			if val, err := strconv.Atoi(wid); err == nil {
				wIDInt = val
			}

			wr, ok := workerRuns[wid]
			if !ok {
				wr = &WorkerRun{WorkerID: wIDInt}
				workerRuns[wid] = wr
			}

			statusStr := "pass"
			if !r.Passed {
				statusStr = "fail"
			} else if r.Total > 0 && r.Skipped == r.Total {
				statusStr = "skip"
			}

			// Relativize path for consistent history
			scriptPath := r.Name
			if strings.HasPrefix(scriptPath, cwd+"/") {
				scriptPath = strings.TrimPrefix(scriptPath, cwd+"/")
			}

			// Relativize log path
			logPath := r.LogPath
			if logPath != "" {
				if strings.HasPrefix(logPath, buildDir+"/") {
					logPath = strings.TrimPrefix(logPath, buildDir+"/")
				}
			}

			wr.Tests = append(wr.Tests, TestRunResult{
				TestPath: scriptPath,
				Status:   statusStr,
				Duration: r.Duration,
				LogPath:  logPath,
			})
		}

		// Convert map to slice
		var workers []WorkerRun
		for _, wr := range workerRuns {
			workers = append(workers, *wr)
		}

		history.AddRun(runID, finalPassed, finalFailed, finalSkipped, workers)
		if err := history.Save(buildDir); err != nil {
			fmt.Printf("Warning: Failed to save test history: %v\n", err)
		}
	}

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		// Clear status line and print interruption
		if isTerminal {
			fmt.Print("\r\033[K")
		}
		fmt.Println("\n\n⚡ Interrupted! Printing partial summary...")

		resultsMu.Lock()
		passed := 0

		// Print failed tests with log links
		for _, r := range results {
			if strings.HasSuffix(r.Name, "/*") {
				passed += (r.Total - r.Failed - r.Skipped)
			} else if r.Passed {
				passed++
			}

			if !r.Passed && r.LogPath != "" {
				fmt.Printf("❌ %-55s -> %s\n", r.Name, r.LogPath)
			}
		}

		// Print in-progress tests (have logs), count never-started
		neverStarted := 0
		for _, t := range testScripts {
			if !completedTests[t] {
				if path, ok := testLogs[t]; ok && path != "" {
					fmt.Printf("⏸  %-55s <in-progress> -> %s\n", t, path)
				} else {
					neverStarted++
				}
			}
		}

		// Save history before exiting
		saveHistory(results)
		resultsMu.Unlock()

		if neverStarted > 0 {
			fmt.Printf("\n(%d tests were never started)\n", neverStarted)
		}
		fmt.Printf("\nPassed: %d/%d (interrupted at %d/%d)\n", passed, len(results), len(results), total)

		if transient {
			client.ShutdownServer(pool)
		}
		os.Exit(130) // Standard exit code for Ctrl+C
	}()

	extraEnv := make(map[string]string)
	if cover {
		coverageDir := filepath.Join(cwd, "build", "coverage")
		os.RemoveAll(coverageDir)
		if err := os.MkdirAll(coverageDir, 0777); err != nil {
			return fmt.Errorf("failed to create coverage dir: %w", err)
		}
		os.Chmod(coverageDir, 0777)
		// GOCOVERDIR must be absolute inside the VM.
		// /mnt/flywall/build/coverage is the standard mount point.
		extraEnv["GOCOVERDIR"] = "/mnt/flywall/build/coverage"
		fmt.Printf("Coverage collection enabled (Output: %s)\n", coverageDir)
	}

	// Background ticker for live UI updates
	stopTicker := make(chan struct{})
	if isTerminal && !verbose {
		ticker := time.NewTicker(500 * time.Millisecond)
		go func() {
			for {
				select {
				case <-ticker.C:
					printStatus()
				case <-stopTicker:
					ticker.Stop()
					return
				}
			}
		}()
	}

	// Handle stress mode defaults
	if stressMode == "stress" {
		if stressDuration == 0 {
			stressDuration = 5 * time.Minute
			fmt.Println("Stress mode enabled: Defaulting to 5m duration")
		} else {
			fmt.Printf("Stress mode enabled: Running for %v\n", stressDuration)
		}
	} else {
		stressDuration = 0
	}

	runStart := time.Now()
	err = client.RunTests(runID, testInfos, logDir, extraEnv, stressDuration, pool, onStart, onOutput, onProgress, onResult)
	runDuration := time.Since(runStart)
	close(stopTicker) // Stop ticker as soon as RunTests returns

	// Clear footer one last time for clean summary
	if isTerminal && !verbose && footerHeight > 0 {
		if footerHeight > 1 {
			fmt.Printf("\033[%dA", footerHeight-1)
		}
		fmt.Print("\r\033[J")
		footerHeight = 0
	}

	if err != nil {
		return err
	}

	// Summary
	passed := 0
	sumSkipped := 0
	sumFailed := 0
	cumulativeDuration := 0.0 // Seconds
	var failed []protocol.TestResult
	for _, r := range results {
		if r.IsSubtest {
			continue // Already accounted for in envelope
		}

		if !r.Passed {
			failed = append(failed, r)
		}

		passed += r.TasksPassed
		sumSkipped += r.TasksSkipped
		sumFailed += r.TasksFailed
		cumulativeDuration += r.Duration.Seconds()
	}

	// Print failed tests with log links
	if len(failed) > 0 {
		fmt.Println("\nFailed tests:")
		for _, r := range failed {
			fmt.Printf("  ❌ %s\n", r.Name)
			if r.LogPath != "" {
				// Make path relative to project root
				relPath := r.LogPath
				if strings.HasPrefix(r.LogPath, cwd) {
					relPath = strings.TrimPrefix(r.LogPath, cwd+"/")
				}
				fmt.Printf("     └─ %s\n", relPath)
			}
		}
	}

	concurrencyRatio := 1.0
	if runDuration.Seconds() > 0 {
		concurrencyRatio = cumulativeDuration / runDuration.Seconds()
	}

	fmt.Printf("\nPassed: %d/%d", passed, total)
	if sumSkipped > 0 {
		fmt.Printf(" (Skipped: %d)", sumSkipped)
	}
	fmt.Printf(" | Total Time: %s | Concurrency: %.1fx", runDuration.Round(100*time.Millisecond), concurrencyRatio)
	fmt.Println()

	// Print artifact directory location if artifacts were saved
	if artifactDir != "" {
		fmt.Printf("\nTest artifacts saved to: %s\n", artifactDir)
	}

	// Save History
	saveHistory(results)

	return nil
}

func runStatus(args []string) error {
	pool := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--pool" && i+1 < len(args) {
			pool = args[i+1]
			break
		}
	}
	resp, err := client.GetStatus(pool)
	if err != nil {
		if strings.Contains(err.Error(), "connect") || strings.Contains(err.Error(), "no such file") {
			return fmt.Errorf("server is not running")
		}
		return fmt.Errorf("failed to get status: %w", err)
	}

	fmt.Println("Orca Server Status")
	fmt.Println("------------------")
	if len(resp.VMs) == 0 {
		fmt.Println("No active workers.")
		return nil
	}

	fmt.Printf("%-6s %-12s %-6s %-6s %-10s %-8s %-12s %s\n", "ID", "STATUS", "BUSY", "JOBS", "MEM(MB)", "LOAD", "LAST HEALTH", "LAST JOB")
	for _, v := range resp.VMs {
		busyStr := "no"
		if v.Busy {
			busyStr = "yes"
		}
		lastJob := filepath.Base(v.LastJob)
		if lastJob == "." {
			lastJob = "-"
		}
		fmt.Printf("%-6s %-12s %-6s %-6d %-10d %-8.2f %-12s %s\n", v.ID, v.Status, busyStr, v.ActiveJobs, v.FreeMemMB, v.LoadAvg, v.LastHealth, lastJob)
	}
	return nil
}

func runStatusVsock(args []string) error {
	targetID := ""
	if len(args) > 0 {
		targetID = args[0]
	}

	startCID := uint32(3)
	endCID := uint32(20)

	if targetID != "" {
		id, err := strconv.Atoi(targetID)
		if err != nil {
			return fmt.Errorf("invalid vm id: %s", targetID)
		}
		// VM ID 1 -> CID 3
		cid := uint32(id + 2)
		startCID = cid
		endCID = cid
		fmt.Printf("Checking VM %s (CID %d)...\n", targetID, cid)
	} else {
		fmt.Println("Scanning for VMs on vsock CIDs 3-20...")
	}

	found := 0
	for cid := startCID; cid <= endCID; cid++ {
		conn, err := vsock.Dial(cid, AgentPort, nil)
		if err != nil {
			continue
		}

		found++
		queryVMStatus(conn, fmt.Sprintf("CID %d", cid))
		conn.Close()
	}

	if found == 0 {
		if targetID != "" {
			fmt.Printf("VM %s not found (or unresponsive).\n", targetID)
		} else {
			fmt.Println("No active VMs found.")
		}
	} else {
		fmt.Printf("Found %d active VM(s).\n", found)
	}
	return nil
}

func runStatusUnix(args []string) error {
	targetID := ""
	if len(args) > 0 {
		targetID = args[0]
	}

	// Prefer mux sockets (support concurrent connections)
	sockets, _ := filepath.Glob("/tmp/flywall-vm*-mux.sock")
	if len(sockets) == 0 {
		// Fall back to raw VM sockets
		sockets, _ = filepath.Glob("/tmp/flywall-vm*.sock")
	}

	if len(sockets) == 0 {
		fmt.Println("No active VMs found.")
		return nil
	}

	if targetID == "" {
		fmt.Printf("Found %d socket file(s), checking...\n\n", len(sockets))
	}

	found := 0
	for _, sock := range sockets {
		id := "unknown"
		if parts := strings.Split(sock, "flywall-vm"); len(parts) > 1 {
			id = strings.TrimSuffix(parts[1], ".sock")
			id = strings.TrimSuffix(id, "-mux") // Remove mux suffix
		}

		// Filter if target specified
		if targetID != "" && id != targetID {
			continue
		}

		vmName := fmt.Sprintf("VM %s", id)

		// 1. Try Side-Channel (File) First
		if status, ok := checkFileStatus(id); ok {
			found++
			printStatusBox(vmName, status, "ACTIVE")
			continue
		}

		// 2. Fallback to Socket
		conn, err := net.DialTimeout("unix", sock, 1*time.Second)
		if err != nil {
			// GC Logic: If connection refused or file missing, it's garbage.
			errStr := err.Error()
			isRefused := strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connect: no such file")

			if isRefused {
				// Clean up stale socket
				os.Remove(sock)

				// Clean up stale status file
				_, buildDir := locateBuildDir()
				statusFile := filepath.Join(buildDir, "vm_status", fmt.Sprintf("flywall-vm-%s.status", id))
				os.Remove(statusFile)

				// Do not count as found, do not print.
				continue
			}

			// Real timeout or other error -> Unresponsive possibly still alive
			printStatusBox(vmName, fmt.Sprintf("UNRESPONSIVE - %v", err), "UNRESPONSIVE")
			continue
		}

		found++
		queryVMStatus(conn, vmName)
		conn.Close()
	}

	if found == 0 {
		fmt.Println("No active VMs found.")
	} else {
		fmt.Printf("Found %d active VM(s).\n", found)
	}
	return nil
}

func checkFileStatus(id string) (string, bool) {
	// Hostname is usually flywall-vm-<id>
	// But `runStatusUnix` extracts ID from `flywall-vm<ID>.sock`.
	// Wait, socket name is `/tmp/flywall-vm<ID>.sock`.
	// Hostname inside VM is set by `vm.go`.
	// Usually `flywall-vm-<ID>`.
	// Let's assume standard naming.

	filename := fmt.Sprintf("flywall-vm-%s.status", id)
	_, buildDir := locateBuildDir()
	path := filepath.Join(buildDir, "vm_status", filename)

	stat, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	// Check freshness (5s heartbeat)
	if time.Since(stat.ModTime()) > 6*time.Second {
		return "", false // Stale
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(content), true
}

func printStatusBox(name, agentStatus, state string) {
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  %s: %s\n", name, state)
	fmt.Printf("  Agent: %s\n", agentStatus)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")
}

func queryVMStatus(conn net.Conn, vmName string) {
	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader.ReadString('\n') // Consume HELLO

	// Send PING
	start := time.Now()
	fmt.Fprintf(conn, "PING\n")
	resp, err := reader.ReadString('\n')
	latency := time.Since(start)

	if err != nil {
		status := "Unknown Error: " + err.Error()
		if strings.Contains(err.Error(), "i/o timeout") {
			status = "Unresponsive (Timeout)"
		} else if strings.Contains(err.Error(), "connection refused") {
			status = "Connection Refused (Stale Socket)"
		}

		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		fmt.Printf("  %s: %s\n", vmName, status)
		fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")
		return
	}

	if strings.TrimSpace(resp) != "PONG" {
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		fmt.Printf("  %s: BAD RESPONSE (Expected PONG, got %q)\n", vmName, strings.TrimSpace(resp))
		fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")
		return
	}

	// Query agent status
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	fmt.Fprintf(conn, "STATUS\n")
	statusResp, _ := reader.ReadString('\n')
	agentStatus := strings.TrimSpace(statusResp)

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  %s: ACTIVE (latency: %v)\n", vmName, latency.Round(time.Millisecond))
	fmt.Printf("  Agent: %s\n", agentStatus)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	// Query memory
	memOutput := execCommand(conn, reader, "cat /proc/meminfo | head -5")
	if memOutput != "" {
		fmt.Printf("\n  📊 Memory:\n")
		for _, line := range strings.Split(memOutput, "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("     %s\n", line)
			}
		}
	}

	// Query CPU load
	loadOutput := execCommand(conn, reader, "cat /proc/loadavg")
	if loadOutput != "" {
		fmt.Printf("\n  🔥 Load Average: %s\n", strings.TrimSpace(loadOutput))
	}

	// Query top processes (BusyBox-compatible)
	// Use 'args' to see full command line (e.g. 'sleep 10')
	psOutput := execCommand(conn, reader, "ps -o pid,user,stat,args | head -10")
	if psOutput != "" {
		fmt.Printf("\n  📋 Top Processes:\n")
		for _, line := range strings.Split(psOutput, "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("     %s\n", line)
			}
		}
	}

	fmt.Println()
}

// execCommand sends an EXEC command and reads the output
func execCommand(conn net.Conn, reader *bufio.Reader, cmd string) string {
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err := fmt.Fprintf(conn, "EXEC %s\n", cmd)
	if err != nil {
		return ""
	}

	var output strings.Builder
	inOutput := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSuffix(line, "\n")

		if strings.HasPrefix(line, "--- BEGIN OUTPUT ---") {
			inOutput = true
			continue
		}
		if strings.HasPrefix(line, "--- END OUTPUT") {
			break
		}
		if inOutput {
			output.WriteString(line)
			output.WriteString("\n")
		}
	}
	return strings.TrimSpace(output.String())
}

// formatDuration formats a duration as MM:SS.mmm
func formatDuration(d time.Duration) string {
	totalSeconds := d.Seconds()
	minutes := int(totalSeconds) / 60
	seconds := totalSeconds - float64(minutes*60)
	return fmt.Sprintf("%02d:%06.3f", minutes, seconds)
}

// testLogName converts a script path to a log name (e.g., t/01-sanity/sanity_test.sh -> 01-sanity--sanity)
func testLogName(scriptPath string) string {
	// Extract directory name and test name
	dir := filepath.Dir(scriptPath)
	base := filepath.Base(scriptPath)

	// Get group from directory (e.g., "01-sanity" from "t/01-sanity")
	group := filepath.Base(dir)

	// Get test name without _test.sh suffix
	name := strings.TrimSuffix(base, "_test.sh")

	return group + "--" + name
}

// writeTestLog writes test output to a log file
func writeTestLog(path string, result TestResult) {
	f, err := os.Create(path)
	if err != nil {
		return // Silently ignore log write failures
	}
	defer f.Close()

	// Write header
	fmt.Fprintf(f, "# Test: %s\n", result.Job.ScriptPath)
	fmt.Fprintf(f, "# Worker: %s\n", result.WorkerID)
	fmt.Fprintf(f, "# Start: %s\n", result.StartTime.Format(time.RFC3339))
	fmt.Fprintf(f, "# Duration: %s\n", formatDuration(result.Duration))
	if result.Error != nil {
		fmt.Fprintf(f, "# Status: FAILED\n")
		fmt.Fprintf(f, "# Error: %v\n", result.Error)
	} else if result.Suite != nil && result.Suite.Success() {
		fmt.Fprintf(f, "# Status: PASSED\n")
	} else {
		fmt.Fprintf(f, "# Status: FAILED\n")
	}
	fmt.Fprintf(f, "\n")

	// Write raw TAP output
	if result.RawOutput != "" {
		f.WriteString(result.RawOutput)
	}

	// Write suite details if available
	if result.Suite != nil {
		fmt.Fprintf(f, "\n# --- Test Results ---\n")
		for _, r := range result.Suite.Results {
			if r.Passed {
				fmt.Fprintf(f, "# ok %d - %s\n", r.Number, r.Description)
			} else {
				fmt.Fprintf(f, "# not ok %d - %s\n", r.Number, r.Description)
				for _, d := range r.Diagnostics {
					fmt.Fprintf(f, "#   %s\n", d)
				}
			}
		}
	}
}

// getVMConnection finds an active VM or starts a temporary one
// Returns connection, cleanup function, and error
func getVMConnection() (net.Conn, func(), error) {
	// 1. Try to find existing VM
	socketPath, err := findFirstValidSocket()
	if err == nil {
		fmt.Printf("Connected to active VM at %s\n", socketPath)
		conn, err := net.DialTimeout("unix", socketPath, 1*time.Second)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to dial existing VM: %w", err)
		}
		return conn, func() { conn.Close() }, nil
	}

	// 2. No VM found, start one
	fmt.Println("No active VM found, configuring temporary session...")

	// Use default config
	projectRoot, buildDir := locateBuildDir()
	cfg := vmm.Config{
		KernelPath:    filepath.Join(buildDir, "vmlinuz"),
		InitrdPath:    filepath.Join(buildDir, "initramfs"),
		RootfsPath:    filepath.Join(buildDir, "rootfs.qcow2"),
		ProjectRoot:   projectRoot,
		ConsoleOutput: false, // Keep stdout clean for shell/exec
	}

	// Create a temp VM with ID 99 (unlikely to collide with 1-9)
	vmID := 99
	vm, err := vmm.NewVM(cfg, vmID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure VM: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start VM in background
	errCh := make(chan error, 1)
	go func() {
		if err := vm.Start(ctx); err != nil {
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	// Wait for socket to appear
	fmt.Printf("Booting VM (ID %d)... ", vmID)
	socketPath = vm.SocketPath

	// Wait up to 30s for socket
	deadline := time.Now().Add(30 * time.Second)
	connected := false
	var conn net.Conn

	for time.Now().Before(deadline) {
		select {
		case err := <-errCh:
			if err != nil {
				cancel()
				return nil, nil, fmt.Errorf("VM failed to start: %w", err)
			}
			// VM exited unexpectedly
			cancel()
			return nil, nil, fmt.Errorf("VM exited unexpectedly")
		default:
			// Check socket
			if _, err := os.Stat(socketPath); err == nil {
				// Try dialing
				c, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
				if err == nil {
					conn = c
					connected = true
					goto Ready
				}
			}
			time.Sleep(500 * time.Millisecond)
			fmt.Print(".")
		}
	}

Ready:
	if !connected {
		cancel()
		return nil, nil, fmt.Errorf("\ntimeout waiting for VM to start")
	}
	fmt.Println(" Ready!")

	cleanup := func() {
		conn.Close()
		cancel()
		// Wait typically ensures clean shutdown, but we cancel context above.
		// We might want to explicitly wait for cmd to exit?
		// But Stop() in NewVM isn't exposed (wait, vm.Stop is).
		vm.Stop()
		<-errCh // Wait for run to finish
	}

	return conn, cleanup, nil
}

func findFirstValidSocket() (string, error) {
	// Prefer mux sockets
	sockets, _ := filepath.Glob("/tmp/flywall-vm*-mux.sock")
	if len(sockets) == 0 {
		sockets, _ = filepath.Glob("/tmp/flywall-vm*.sock")
	}

	for _, sock := range sockets {
		// Test connectivity
		c, err := net.DialTimeout("unix", sock, 100*time.Millisecond)
		if err == nil {
			c.Close()
			return sock, nil
		}
	}
	return "", fmt.Errorf("no active vm sockets found")
}

func runShell(args []string) error {
	vmid := ""
	pool := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--help" || args[i] == "-h" || args[i] == "help" {
			helpShell()
			return nil
		}
		if (args[i] == "--vmid" || args[i] == "-v") && i+1 < len(args) {
			vmid = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
			continue
		}
		if args[i] == "--pool" && i+1 < len(args) {
			pool = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
			continue
		}
	}
	if _, err := client.EnsureServer(false, 0, 0, false, pool); err != nil {
		return err
	}
	return client.RunShell(vmid, pool)
}

func runExec(args []string) error {
	vmid := ""
	pool := ""
	// Pre-process args looking for --vmid or -v or --pool
	for i := 0; i < len(args); i++ {
		if (args[i] == "--vmid" || args[i] == "-v") && i+1 < len(args) {
			vmid = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
		} else if args[i] == "--pool" && i+1 < len(args) {
			pool = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
		} else if args[i] == "--help" || args[i] == "-h" || args[i] == "help" {
			helpExec()
			return nil
		}
	}

	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: exec <command>")
	}
	if _, err := client.EnsureServer(false, 0, 0, false, pool); err != nil {
		return err
	}
	return client.RunExec(args, false, vmid, pool)
}

// Control file for signaling warm pool shutdown
const warmPoolControlFile = "/tmp/flywall-orca-pool.pid"

func runStop(args []string) error {
	pool := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--pool" && i+1 < len(args) {
			pool = args[i+1]
			break
		}
	}
	fmt.Printf("Shutting down Orca Server (Pool: %s)...\n", pool)
	if err := client.ShutdownServer(pool); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	fmt.Println("Shutdown signal sent.")
	return nil
}

// monitorResources logs system resource usage to a file
func monitorResources(cwd string) {
	logPath := filepath.Join(cwd, "build", "orca-resources.log")
	f, err := os.Create(logPath)
	if err != nil {
		fmt.Printf("Warning: Failed to create resource log: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "Time,Goroutines,HeapAllocMB,SysMB,OpenFiles\n")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Count open files (best effort)
		openFiles := 0
		if fds, err := os.ReadDir("/dev/fd"); err == nil {
			openFiles = len(fds)
		} else if fds, err := os.ReadDir("/proc/self/fd"); err == nil {
			openFiles = len(fds)
		}

		fmt.Fprintf(f, "%s,%d,%d,%d,%d\n",
			time.Now().Format(time.RFC3339),
			runtime.NumGoroutine(),
			m.HeapAlloc/1024/1024,
			m.Sys/1024/1024,
			openFiles,
		)
	}
}

func CalculateOptimalWorkers(quiet bool) (int, int) {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}

	load, freeMB := getSystemStats()
	// Only print stats if we found them (freeMB > 0)
	if freeMB > 0 && !quiet {
		fmt.Printf("System Stats: CPUs=%d, Load=%.2f, FreeMem=%dMB\n", n, load, freeMB)
	}

	// Base max is NumCPU
	max := n

	// Adjust for Load (if available) - checking load > 0.0 to avoid 0.0 default issues
	if load > 0.0 && load > float64(n)*1.5 {
		// High load
		max = int(float64(max) * 0.6)
	}

	// Adjust for Memory (estimated 350MB per runner: 256MB VM + ~80MB overhead)
	if freeMB > 0 {
		// Reserve 2GB for System/Other apps
		availableForTest := freeMB - 2048
		if availableForTest < 350 {
			availableForTest = 350 // Keep at least one runner valid
		}
		memMax := availableForTest / 350
		if memMax < 1 {
			memMax = 1
		}

		if memMax < max {
			max = memMax
		}
	}

	// Hard caps
	if max > 16 {
		max = 16
	}
	if max < 1 {
		max = 1
	}

	// Warm pool logic
	warm := max / 2
	if warm < 2 && max >= 2 {
		warm = 2
	}
	if warm > max {
		warm = max
	}

	return warm, max
}

func getSystemStats() (float64, int) {
	if runtime.GOOS == "darwin" {
		return getDarwinStats()
	} else if runtime.GOOS == "linux" {
		return getLinuxStats()
	}
	return 0, 0
}

func getDarwinStats() (float64, int) {
	// Load
	out, _ := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	// { 1.23 4.56 ... }
	s := strings.Trim(string(out), "{ } \n")
	parts := strings.Fields(s)
	load := 0.0
	if len(parts) > 0 {
		load, _ = strconv.ParseFloat(parts[0], 64)
	}

	// Mem
	out, _ = exec.Command("sysctl", "-n", "hw.pagesize").Output()
	pageSize := 4096
	if ps, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
		pageSize = ps
	}

	out, _ = exec.Command("vm_stat").Output()
	// Pages free: 123.
	// Pages inactive: 456.
	// Pages speculative: 789.
	lines := strings.Split(string(out), "\n")
	freePages := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "Pages free:") || strings.HasPrefix(line, "Pages inactive:") || strings.HasPrefix(line, "Pages speculative:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				num, _ := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(parts[1], ".")))
				freePages += num
			}
		}
	}

	freeMB := (freePages * pageSize) / 1024 / 1024
	return load, freeMB
}

func getLinuxStats() (float64, int) {
	// Load
	data, _ := os.ReadFile("/proc/loadavg")
	parts := strings.Fields(string(data))
	load := 0.0
	if len(parts) > 0 {
		load, _ = strconv.ParseFloat(parts[0], 64)
	}

	// Mem
	data, _ = os.ReadFile("/proc/meminfo")
	lines := strings.Split(string(data), "\n")
	availKB := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "MemAvailable:") {
			// MemAvailable:    123456 kB
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				availKB, _ = strconv.Atoi(parts[1])
			}
		}
	}
	return load, availKB / 1024
}

// computeDisplayNames calculates the minimum unique suffix for a set of paths
func computeDisplayNames(paths []string) map[string]string {
	displayMap := make(map[string]string)

	type info struct {
		path       string
		components []string
		display    string
	}

	infos := make([]*info, len(paths))
	for i, p := range paths {
		// Clean path and split
		clean := filepath.Clean(p)
		parts := strings.Split(clean, string(os.PathSeparator))
		infos[i] = &info{path: p, components: parts}
	}

	for _, inf := range infos {
		suffixLen := 1
		for {
			if suffixLen > len(inf.components) {
				inf.display = inf.path // Fallback to full path
				break
			}

			// Construct candidate suffix
			start := len(inf.components) - suffixLen
			candidate := strings.Join(inf.components[start:], string(os.PathSeparator))

			// Check uniqueness against all OTHER paths
			unique := true
			for _, other := range infos {
				if other == inf {
					continue
				}
				// Ambiguity check:
				// A candidate is ambiguous if it ALSO appears as a suffix in another path.
				// e.g. "bar.sh" is suffix of "foo/bar.sh" AND "baz/bar.sh".
				// So if we used "bar.sh" for "foo/bar.sh", users might confuse it with "baz/bar.sh".
				// So we check if 'candidate' is a suffix of 'other.path'.

				// Reconstruct other path from components to be safe
				otherPathDesc := strings.Join(other.components, string(os.PathSeparator))
				if strings.HasSuffix(otherPathDesc, string(os.PathSeparator)+candidate) || otherPathDesc == candidate {
					unique = false
					break
				}
			}

			if unique {
				inf.display = candidate
				break
			}
			suffixLen++
		}
		displayMap[inf.path] = inf.display
	}
	return displayMap
}

func helpDemo() {
	fmt.Println("Usage: orca demo")
	fmt.Println("\nStarts the Flywall VM in Live Demo mode.")
	fmt.Println("  - Maps Host 8080 -> Guest 8080 (HTTP)")
	fmt.Println("  - Maps Host 8443 -> Guest 8443 (HTTPS)")
	fmt.Println("  - Attaches interactive console (login: root)")
	fmt.Println("  - Uses Copy-On-Write overlay to preserve state")
}

func runDemo(args []string) error {
	// Create RawModeWriter for proper output formatting (inject \r)
	// We use this for ALL output to ensure correct framing even if terminal is already raw.
	rawOut := &RawModeWriter{Target: os.Stdout}

	// Default ports
	httpPort := 8080
	httpsPort := 8443
	sshPort := 8022

	// Parse flags early for UI output correctness
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--port" || arg == "-p" {
			if i+1 < len(args) {
				if val, err := strconv.Atoi(args[i+1]); err == nil && val > 0 {
					httpsPort = val
					// Auto-assign HTTP port if not explicitly set (to avoid 8080 collision)
					if httpPort < 1024 {
						httpPort = 0
					}
				}
			}
		}
		if arg == "--http-port" || arg == "--http" {
			if i+1 < len(args) {
				if val, err := strconv.Atoi(args[i+1]); err == nil && val > 0 {
					httpPort = val
				}
			}
		}
		if arg == "--https" {
			if i+1 < len(args) {
				if val, err := strconv.Atoi(args[i+1]); err == nil && val > 0 {
					httpsPort = val
				}
			}
		}
		if arg == "--ssh" {
			if i+1 < len(args) {
				if val, err := strconv.Atoi(args[i+1]); err == nil && val > 0 {
					sshPort = val
				}
			}
		}
	}

	// Auto-increment default ports if they are in use?
	// Or simplistic approach: if user specifies --ssh 8888, they expect unique ports
	// but we defaulted http/https to 8080/8443 which collide with running instance.
	// We need a way to shift ALL ports if not specified, OR warn.
	// Users usually run: `fw demo --ssh 8888 --http 8081 --https 8444` to avoid collision.
	// Let's rely on user explicit configuration for now, but print a helpful error if qemu fails?
	// The problem is QEMU fail is hard to parse.

	// Better: Checking availability before starting?
	// Check TCP listen on host ports.
	checkPort := func(p int) bool {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err != nil {
			return false
		}
		ln.Close()
		return true
	}

	if !checkPort(httpPort) {
		fmt.Fprintf(rawOut, "Warning: Port %d is in use. Auto-selecting new port...\n", httpPort)
		// Find free port
		ln, _ := net.Listen("tcp", ":0")
		if ln != nil {
			httpPort = ln.Addr().(*net.TCPAddr).Port
			ln.Close()
		}
	}
	if !checkPort(httpsPort) {
		fmt.Fprintf(rawOut, "Warning: Port %d is in use. Auto-selecting new port...\n", httpsPort)
		ln, _ := net.Listen("tcp", ":0")
		if ln != nil {
			httpsPort = ln.Addr().(*net.TCPAddr).Port
			ln.Close()
		}
	}
	// Check SSH port too
	if !checkPort(sshPort) {
		fmt.Fprintf(rawOut, "Warning: Port %d is in use. Auto-selecting new port...\n", sshPort)
		ln, _ := net.Listen("tcp", ":0")
		if ln != nil {
			sshPort = ln.Addr().(*net.TCPAddr).Port
			ln.Close()
		}
	}

	// Check for --headless flag and --config
	headless := false
	runDir := "" // Optional per-run artifact directory
	userConfigPath := ""
	for i, arg := range args {
		if arg == "--headless" {
			headless = true
		}
		if arg == "--run-dir" && i+1 < len(args) {
			runDir = args[i+1]
		}
		if (arg == "--config" || arg == "-c") && i+1 < len(args) {
			userConfigPath = args[i+1]
		}
	}

	fmt.Fprintln(rawOut, "Flywall Live Demo Starting...")

	projectRoot, buildDir := locateBuildDir()

	// 1. Generate Demo Config (flywall.demo.hcl)
	// We read flywall.hcl from project root, and check if it exists
	configSource := filepath.Join(projectRoot, "flywall.hcl")
	if userConfigPath != "" {
		if filepath.IsAbs(userConfigPath) {
			configSource = userConfigPath
		} else {
			// Resolve relative to CWD, not project root, as it is a user arg
			cwd, _ := os.Getwd()
			configSource = filepath.Join(cwd, userConfigPath)
		}
	}

	// Determine where to write the config and what VM path to use
	var demoConfigPath string

	if runDir == "" {
		// Generate unique Run ID and directory
		runID := fmt.Sprintf("run-%d-%04d", time.Now().Unix(), rand.Intn(10000))
		runDir = filepath.Join(buildDir, "runs", runID)
		os.MkdirAll(runDir, 0755)
		fmt.Fprintf(rawOut, "Created new isolated run directory: %s\n", runDir)
	}

	// Always use the isolated run directory
	demoConfigPath = filepath.Join(runDir, "flywall.demo.hcl")
	// VM path is now always /opt/flywall/flywall.demo.hcl because we mount runDir to /opt/flywall
	// vmConfigPath variable is deprecated by hardcoded /opt/flywall path in start command

	pidFile := filepath.Join(buildDir, "demo.pid")

	// Check if already running
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if proc, err := os.FindProcess(pid); err == nil {
				// Signal(0) checks if process exists and we have permission
				if err := proc.Signal(syscall.Signal(0)); err == nil {
					// Send SIGUSR1 to trigger upgrade in the running instance
					if err := proc.Signal(syscall.SIGUSR1); err == nil {
						fmt.Fprintf(rawOut, "Demo is already running (PID %d).\nUpgrade signal (SIGUSR1) sent to trigger hot reload.\n", pid)
					} else {
						fmt.Fprintf(rawOut, "Demo is already running (PID %d), but failed to send signal: %v\n", pid, err)
					}
					return nil
				}
			}
		}
		// If we get here, PID file is stale
		os.Remove(pidFile)
	}

	if content, err := os.ReadFile(configSource); err == nil {
		// Verify schema version to avoid breaking changes (optional, but good practice)
		// We'll just perform simple string manipulation or append our overrides
		// HCL allows overriding by appending later in the file or just trusting the user?
		// User specifically asked to "make the log locations configurable".
		// We already enabled LogDir in Config.
		// So we can append overrides to the configuration.

		// Create the demo config
		f, err := os.Create(demoConfigPath)
		if err != nil {
			return fmt.Errorf("failed to create demo config: %w", err)
		}

		// Inject tls_listen into existing web block rather than adding a new block
		// (HCL doesn't allow duplicate blocks)
		modifiedContent := string(content)
		// Find "web {" and insert tls_listen right after it
		if strings.Contains(modifiedContent, "web {") {
			modifiedContent = strings.Replace(modifiedContent, "web {", "web {\n    tls_listen = \":8443\"", 1)
		} else {
			// If no web block, append one
			modifiedContent += "\nweb {\n  tls_listen = \":8443\"\n  listen = \":8080\"\n}\n"
		}

		f.WriteString(modifiedContent)
		f.WriteString("\n\n# --- Demo Overrides ---\n")

		// Use per-run artifact directories (relative to /opt/flywall in VM)
		hostStateDir := filepath.Join(runDir, "state")
		hostLogDir := filepath.Join(runDir, "logs")

		// Create directories
		if err := os.MkdirAll(hostStateDir, 0777); err != nil {
			fmt.Fprintf(rawOut, "Warning: Failed to create state dir: %v\n", err)
		}
		if err := os.MkdirAll(hostLogDir, 0777); err != nil {
			fmt.Fprintf(rawOut, "Warning: Failed to create log dir: %v\n", err)
		}

		// Force 777 permissions (MkdirAll is affected by umask)
		os.Chmod(runDir, 0777)
		os.Chmod(hostStateDir, 0777)
		os.Chmod(hostLogDir, 0777)

		// Inside VM, runDir is mounted at /opt/flywall
		f.WriteString("state_dir = \"/opt/flywall/state\"\n")
		f.WriteString("log_dir = \"/opt/flywall/logs\"\n")

		// Configure eth0 with DHCP
		f.WriteString("\ninterface \"eth0\" {\n  dhcp_client = \"native\"\n}\n")
		f.Close()
		fmt.Fprintf(rawOut, "Using config: %s\n", configSource)
		fmt.Fprintf(rawOut, "Generated demo config at %s\n", demoConfigPath)
	} else {
		return fmt.Errorf("config file not found at %s: %w", configSource, err)
	}

	cfg := vmm.Config{
		KernelPath:     filepath.Join(buildDir, "vmlinuz"),
		InitrdPath:     filepath.Join(buildDir, "initramfs"),
		RootfsPath:     filepath.Join(buildDir, "rootfs.qcow2"),
		ProjectRoot:    projectRoot,
		Debug:          false, // Not debug mode, but we want console
		BuildDir:       buildDir,
		BuildSharePath: runDir, // ISOLATION: Mount the run directory as build_share
		DevMode:        false,  // Explicit process management via Exec
		InterfaceCount: 6,      // Request rich hardware environment (1 WAN + 5 LAN)
		ForwardPorts: map[int]int{
			httpPort:  8080,
			httpsPort: 8443,
			sshPort:   2222, // SSH Forwarding
		},
	}

	// Handle input hijacking for Ctrl-C
	// We want to send input to VM, but also detect Ctrl-C to kill the Host process
	// because users intuitively expect Ctrl-C to exit the demo.

	// Use os.Pipe instead of io.Pipe for kernel buffering (avoids blocking read loop if VM is slow)
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	cfg.Stdin = stdinReader

	// Create logs directory
	logDir := buildDir
	if runDir != "" {
		logDir = runDir
		os.MkdirAll(logDir, 0755)
	}

	// 1. VM Console Log
	vmLogPath := filepath.Join(logDir, "demo_vm.log")
	vmLogFile, err := os.Create(vmLogPath)
	if err != nil {
		return fmt.Errorf("failed to create vm log: %w", err)
	}
	defer vmLogFile.Close()

	cfg.Stdout = vmLogFile
	cfg.Stderr = vmLogFile

	// 2. Forwarder Log
	fwLogPath := filepath.Join(logDir, "demo_forwarder.log")
	fwLogFile, err := os.Create(fwLogPath)
	if err != nil {
		return fmt.Errorf("failed to create forwarder log: %w", err)
	}
	defer fwLogFile.Close()

	vm, err := vmm.NewVM(cfg, 1) // ID 1
	if err != nil {
		return err
	}
	defer vm.Stop()

	fmt.Fprintln(rawOut, "\n"+strings.Repeat("=", 60))
	fmt.Fprintln(rawOut, "  Flywall Demo Environment")
	fmt.Fprintln(rawOut, "  "+strings.Repeat("=", 58))
	fmt.Fprintf(rawOut, "  ➜  Web UI:  http://localhost:%d\n", httpPort)
	fmt.Fprintf(rawOut, "  ➜  HTTPS:   https://localhost:%d\n", httpsPort)
	fmt.Fprintf(rawOut, "  ➜  SSH:     ssh -p %d localhost\n", sshPort)
	fmt.Fprintf(rawOut, "  ➜  Logs:    %s\n", vmLogPath)
	fmt.Fprintln(rawOut, strings.Repeat("=", 60)+"\n")

	// Set Raw Mode (moved to main thread to ensure Restore happens on exit)
	// Skip in headless mode - no terminal interaction needed
	if !headless {
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			oldState, err := term.MakeRaw(fd)
			if err == nil {
				defer term.Restore(fd, oldState)
			}
		}
	}

	// Handle signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals (revamped for SIGUSR1 support)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)
	go func() {
		for sig := range sigCh {
			if sig == syscall.SIGUSR1 {
				fmt.Fprintf(rawOut, "\r\n[Signal] Force upgrade requested via SIGUSR1... performing hot reload.\r\n")

				// Upgrade Logic
				socketPath := filepath.Join(vm.Config.BuildDir, "flywall-demo.sock")

				// 1. Copy to _new location
				if err := client.RunExecWithSocket([]string{"cp", "/mnt/flywall/build/flywall", "/usr/sbin/flywall_new"}, false, "", socketPath, rawOut, rawOut); err != nil {
					fmt.Fprintf(rawOut, "\r\n[Signal] Upgrade copy failed: %v\r\n", err)
					continue
				}

				// 2. Trigger Upgrade
				if err := client.RunExecWithSocket([]string{"flywall", "upgrade"}, false, "", socketPath, rawOut, rawOut); err != nil {
					fmt.Fprintf(rawOut, "\r\n[Signal] Upgrade command failed: %v\r\n", err)
				} else {
					fmt.Fprintf(rawOut, "\r\n[Signal] Upgrade command sent.\r\n")
				}
				continue
			}

			// Shutdown signals
			fmt.Fprintf(rawOut, "\r\nStopping Demo VM...\r\n")
			cancel()
			return
		}
	}()

	// Stdin Proxy - only run in interactive mode
	if !headless {
		go func() {
			defer stdinWriter.Close() // Ensure EOF is sent to VM stdin wrapper so Wait() returns
			buf := make([]byte, 1024)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil {
					return
				}

				// Detect Ctrl-C (ETX = 0x03)
				for i := 0; i < n; i++ {
					if buf[i] == 3 {
						// Cancel context to kill VM
						fmt.Fprintf(rawOut, "\r\nCaught Ctrl-C. Exiting...\r\n")
						cancel()
						return
					}
				}

				if _, err := stdinWriter.Write(buf[:n]); err != nil {
					return
				}
			}
		}()
	} else {
		// In headless mode, close stdin pipe immediately so VM gets EOF
		// and doesn't block waiting for input
		stdinWriter.Close()
	}
	// Save PID to file for 'orca demo stop'
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	defer os.Remove(pidFile)

	// Start Agent Port Forwarding (Background)
	pf := startDemoPortForwarding(vm, fwLogFile, httpPort, httpsPort)

	// Wait for Agent
	go func() {
		// TUI Status Helper
		printStatus := func(msg string) {
			if !headless {
				// Clear line and print message with \r to return to start
				// padding to overwrite previous long messages
				fmt.Fprintf(rawOut, "\r%-70s", msg)
			}
		}

		printStatus("Waiting for Agent connection...")
		pf.WaitReady()
		printStatus("Agent Connected! Bootstrapping Flywall...")

		socketPath := filepath.Join(vm.Config.BuildDir, "flywall-demo.sock")

		// Determine correct binary path
		// VM is Linux, so we need the Linux binary.
		// Host could be macOS (Darwin) or Linux.
		// We assume the VM architecture matches the Host architecture (virt framework/kvm restriction).
		arch := runtime.GOARCH
		binaryName := fmt.Sprintf("flywall-linux-%s", arch)
		// Check if it exists, otherwise fallback to "flywall" (if user just built native on linux)
		sourceBinaryPath := filepath.Join(vm.Config.BuildDir, binaryName)

		if _, err := os.Stat(sourceBinaryPath); err != nil {
			printStatus(fmt.Sprintf("Warning: %s not found. Checking 'flywall'...", binaryName))
			if _, err := os.Stat(filepath.Join(vm.Config.BuildDir, "flywall")); err == nil {
				binaryName = "flywall"
				sourceBinaryPath = filepath.Join(vm.Config.BuildDir, "flywall")
			} else {
				fmt.Fprintf(rawOut, "\r\nError: No suitable binary found. Please run 'fw build linux'.\r\n")
				return
			}
		}

		// ISOLATION: Hardlink the binary into the run directory
		// We use hardlink to save space/time, but symlink might fail inside 9p share if valid logic isn't there.
		// Copy is safest for VM shares, but hardlink is good if on same FS.
		// Let's try Copy for robustness across filesystems/mounts, or Hardlink if possible.
		// Start with Hardlink, fallback to Copy.
		runBinaryPath := filepath.Join(runDir, "flywall")
		os.Remove(runBinaryPath) // Ensure clean state
		if err := os.Link(sourceBinaryPath, runBinaryPath); err != nil {
			// Fallback to copy
			input, err := os.ReadFile(sourceBinaryPath)
			if err == nil {
				os.WriteFile(runBinaryPath, input, 0755)
			}
		}

		// Helper for quiet execution
		execQuiet := func(cmd []string) error {
			var buf bytes.Buffer
			// Capture both stdout and stderr
			err := client.RunExecWithSocket(cmd, false, "", socketPath, &buf, &buf)
			if err != nil {
				// On error, print the captured output for debugging
				fmt.Fprintf(rawOut, "\r\nCommand failed: %v\nOutput:\n%s\n", cmd, buf.String())
				return err
			}
			return nil
		}

		// 1. Mount Run Directory (Isolation)
		// We mount the specific run directory to /opt/flywall
		execQuiet([]string{"mkdir", "-p", "/opt/flywall"})
		execQuiet([]string{"mount", "-t", "9p", "-o", "trans=virtio,version=9p2000.L", "build_share", "/opt/flywall"})

		// 2. Install/Link Binary?
		// Since we mounted the run dir to /opt/flywall, the binary is already at /opt/flywall/flywall
		// We symlink it to /usr/sbin/flywall for convenience (and systemd units if we had them)
		execQuiet([]string{"ln", "-sf", "/opt/flywall/flywall", "/usr/sbin/flywall"})
		execQuiet([]string{"chmod", "+x", "/opt/flywall/flywall"})

		// 3. Network Setup
		execQuiet([]string{"ip", "link", "set", "up", "dev", "lo"})
		// execQuiet([]string{"ip", "addr", "add", "127.0.0.1/8", "dev", "lo"})

		// 4. Start Flywall
		printStatus("Starting Flywall Daemon...")

		// Config is at /opt/flywall/flywall.demo.hcl (because we wrote it to runDir/flywall.demo.hcl)
		cmd := []string{"flywall", "start", "-c", "/opt/flywall/flywall.demo.hcl"}

		// Use execQuiet for start command too to suppress output
		// Assuming 'flywall start' doesn't block indefinitely without backgrounding.
		// If it blocks, execQuiet buffers forever.
		// But in previous code, we waited for RunExecWithSocket.
		// So we assume it returns.
		if err := execQuiet(cmd); err != nil {
			fmt.Fprintf(rawOut, "Failed to start Flywall.\r\n")
		} else {
			printStatus("✔ Flywall stack started successfully.")
			fmt.Fprintf(rawOut, "\r\n") // Final newline
		}

		// 5. Start Watcher (Host Side)
		go func() {
			binPath := sourceBinaryPath

			// Get initial mod time
			lastMod := time.Time{}
			if info, err := os.Stat(binPath); err == nil {
				lastMod = info.ModTime()
			}

			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Check for change
					if info, err := os.Stat(binPath); err == nil {
						if info.ModTime().After(lastMod) {
							// Ensure file is fully written and stable
							if time.Since(info.ModTime()) < 2*time.Second {
								continue
							}
							if info.Size() == 0 {
								// Ignore empty files (build start?)
								continue
							}

							fmt.Fprintf(rawOut, "\r\n[Watcher] Detected binary change (Size: %d bytes). Upgrading...\r\n", info.Size())
							lastMod = info.ModTime()

							// Perform Upgrade
							// 1. Copy to _new location (expected by upgrade command for no-arg run)
							// The target path in the VM is /opt/flywall/flywall
							// BUT, for upgrade, we want to copy the HOST binary to the GUEST /usr/sbin/flywall_new
							// The 'cp' command in RunExecWithSocket runs inside the GUEST.
							// So valid guest paths: /opt/flywall/flywall (which is the mounted run directory)
							// WAIT! If we update the host binary (sourceBinaryPath), we MUST ALSO update the
							// hardlinked/copied binary in the run directory (runBinaryPath) because that's what is mounted!

							// UPDATE RUN DIR BINARY FIRST
							runBinaryPath := filepath.Join(runDir, "flywall")
							input, err := os.ReadFile(sourceBinaryPath)
							if err == nil {
								os.WriteFile(runBinaryPath, input, 0755)
							}

							// Now copy from the MOUNTED path (/opt/flywall/flywall) to /usr/sbin/flywall_new
							cmd := []string{"cp", "/opt/flywall/flywall", "/usr/sbin/flywall_new"}
							if err := client.RunExecWithSocket(cmd, false, "", socketPath, rawOut, rawOut); err != nil {
								fmt.Fprintf(rawOut, "\r\n[Watcher] Copy failed: %v\r\n", err)
								continue
							}

							// 2. Trigger Upgrade
							if err := client.RunExecWithSocket([]string{"flywall", "upgrade"}, false, "", socketPath, rawOut, rawOut); err != nil {
								fmt.Fprintf(rawOut, "\r\n[Watcher] Upgrade command failed: %v\r\n", err)
							} else {
								fmt.Fprintf(rawOut, "\r\n[Watcher] Upgrade command sent.\r\n")
							}
						}
					}
				}
			}
		}()
	}()

	if err := vm.Start(ctx); err != nil {
		return fmt.Errorf("demo error: %w", err)
	}

	return nil
}

func runDemoStop(args []string) error {
	fmt.Println("Stopping Flywall Demo...")

	_, buildDir := locateBuildDir()
	pidFile := filepath.Join(buildDir, "demo.pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no demo running (pid file not found at %s)", pidFile)
		}
		return fmt.Errorf("failed to read pid file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid pid in file: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Verify if it's already gone
		if err == os.ErrProcessDone {
			fmt.Println("Demo process already stopped")
			os.Remove(pidFile)
			return nil
		}
		return fmt.Errorf("failed to signal process: %w", err)
	}

	fmt.Printf("Sent SIGTERM to orca demo (PID %d)\n", pid)
	return nil
}

func getVisualWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 127 {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func padRight(s string, width int) string {
	vw := getVisualWidth(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}

func getVisualLineCount(s string) int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 1 // Fallback
	}
	vw := getVisualWidth(s)
	if vw == 0 {
		return 1
	}
	return (vw + width - 1) / width
}

func runUnitTest(args []string) error {
	projectRoot, buildDir := locateBuildDir()

	// Parse arguments
	var binDir string
	var testArgs []string
	var pool string

	var filter string
	for i := 0; i < len(args); i++ {
		if args[i] == "--bin-dir" {
			if i+1 < len(args) {
				binDir = args[i+1]
				i++
			}
			continue
		}
		if args[i] == "--pool" {
			if i+1 < len(args) {
				pool = args[i+1]
				i++
			}
			continue
		}
		if args[i] == "-filter" {
			if i+1 < len(args) {
				filter = args[i+1]
				i++
			}
			continue
		}
		testArgs = append(testArgs, args[i])
	}

	// Prepare final test args - map -filter to -test.run if provided
	finalTestArgs := testArgs
	if filter != "" {
		finalTestArgs = append(finalTestArgs, "-test.run", filter)
	}

	// If pool is specified, try to use it instead of starting a new VM
	if pool != "" {
		if _, err := client.EnsureServer(false, 0, 0, false, pool); err != nil {
			return fmt.Errorf("failed to ensure orca pool %s: %w", pool, err)
		}

		// List test binaries in host binDir
		files, err := os.ReadDir(binDir)
		if err != nil {
			return fmt.Errorf("failed to read bin dir %s: %w", binDir, err)
		}

		fmt.Printf("Using Orca Pool: %s\n", pool)
		var failures int
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".test") {
				vmPath := filepath.Join("/mnt/build", f.Name())
				// We assume standard pool VMs mount build dir to /mnt/build or similar.
				// Actually, standard VMs mount project root to /mnt/flywall.
				// And building usually happens in project root / build.
				// So if binDir is relative to project root, we can map it.
				absBinDir, _ := filepath.Abs(binDir)
				absProjectRoot, _ := filepath.Abs(projectRoot)
				relBinDir, err := filepath.Rel(absProjectRoot, absBinDir)
				if err == nil {
					vmPath = filepath.Join("/mnt/flywall", relBinDir, f.Name())
				}

				if filter != "" {
					// Smart filter: if binary name matches filter, or if filter doesn't look like a package name, run it.
					// For now, let's just run it if name contains filter OR if we're not sure.
					if !strings.Contains(f.Name(), filter) {
						// But if the filter is "TestSomething", it won't be in the package name.
						// So we only skip if the filter looks like it COULD be a package name.
						// A simple heuristic: if it doesn't start with "Test", it might be a package.
						if !strings.HasPrefix(filter, "Test") {
							continue
						}
					}
				}

				fmt.Printf("Running Test: %s\n", f.Name())
				cmd := []string{vmPath}
				if len(finalTestArgs) > 0 {
					cmd = append(cmd, finalTestArgs...)
				} else {
					cmd = append(cmd, "-test.v")
				}

				if err := client.RunExec(cmd, false, "", pool); err != nil {
					fmt.Printf("Test FAILED: %s: %v\n", f.Name(), err)
					failures++
				}
			}
		}

		if failures > 0 {
			return fmt.Errorf("%d test packages failed", failures)
		}
		return nil
	}

	// Create a temporary directory for unique socket/overlay
	tempDir := filepath.Join(buildDir, "temp-vm-runs")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Use a randomized ID to avoid collision with standard pool (1000+)
	vmID := 1000 + (os.Getpid() % 10000)

	cfg := vmm.Config{
		KernelPath:  filepath.Join(buildDir, "vmlinuz"),
		InitrdPath:  filepath.Join(buildDir, "initramfs"),
		RootfsPath:  filepath.Join(buildDir, "rootfs.qcow2"),
		ProjectRoot: projectRoot,
		BuildDir:    tempDir,
		MemoryMB:    1024,
		Debug:       false, // Keep stdout clean for test output
		DevMode:     false, // Use agent mode
	}

	vm, err := vmm.NewVM(cfg, vmID)
	if err != nil {
		return fmt.Errorf("failed to create vm: %w", err)
	}
	defer vm.Stop()

	// Capture signals to stop VM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		vm.Stop()
		os.Exit(1)
	}()

	fmt.Printf("Starting Test VM (ID %d)...\n", vmID)

	// Run VM in goroutine as vm.Start blocks until exit
	vmErrCh := make(chan error, 1)
	go func() {
		vmErrCh <- vm.Start(ctx)
	}()

	// Determine socket path
	socketPath := vm.SocketPath

	// Wait for agent connection and establish persistent session
	var conn net.Conn

	fmt.Print("Waiting for VM Agent...")
	start := time.Now()
	connected := false

setupLoop:
	for time.Since(start) < 30*time.Second {
		select {
		case err := <-vmErrCh:
			fmt.Println()
			return fmt.Errorf("vm exited prematurely: %w", err)
		default:
		}

		c, err := net.Dial("unix", socketPath)
		if err == nil {
			conn = c
			connected = true
			fmt.Println(" Connected.")
			break setupLoop
		}
		time.Sleep(500 * time.Millisecond)
		fmt.Print(".")
	}
	fmt.Println()

	if !connected {
		return fmt.Errorf("timed out waiting for agent connection at %s", socketPath)
	}
	defer conn.Close()

	// Setup Protocol Streams
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	// Helper to exec command directly on Agent via protocol
	execAgent := func(cmd []string, timeout time.Duration, capture bool) (string, error) {
		// Set deadline based on timeout
		if timeout > 0 {
			conn.SetDeadline(time.Now().Add(timeout))
		} else {
			conn.SetDeadline(time.Time{}) // clear deadline
		}

		reqID := fmt.Sprintf("req-%d", time.Now().UnixNano())
		msg := protocol.Message{
			Type: protocol.MsgExec,
			ID:   reqID,
			Payload: protocol.ExecPayload{
				Command: cmd,
				Tty:     false,
			},
		}

		if err := enc.Encode(msg); err != nil {
			return "", fmt.Errorf("send exec: %w", err)
		}

		var outputBuf bytes.Buffer

		// Read loop
		for {
			var resp protocol.Message
			if err := dec.Decode(&resp); err != nil {
				if err == io.EOF {
					return "", fmt.Errorf("agent closed connection")
				}
				return "", fmt.Errorf("decode error: %w", err)
			}

			// Filter for our request ID
			if resp.Ref != reqID {
				continue
			}

			switch resp.Type {
			case protocol.MsgStdout:
				if capture {
					outputBuf.Write(resp.Data)
				} else {
					os.Stdout.Write(resp.Data)
				}
			case protocol.MsgStderr:
				if capture {
					outputBuf.Write(resp.Data)
				} else {
					os.Stderr.Write(resp.Data)
				}
			case protocol.MsgExit:
				if resp.ExitCode != 0 {
					return outputBuf.String(), fmt.Errorf("exit status %d", resp.ExitCode)
				}
				return outputBuf.String(), nil
			case protocol.MsgError:
				return "", fmt.Errorf("agent error: %s", resp.Error)
			}
		}
	}

	// Mount build_share if binDir is set
	if binDir != "" {
		// Check existing mounts first (30s timeout)
		mounts, err := execAgent([]string{"mount"}, 30*time.Second, true)
		if err != nil {
			return fmt.Errorf("failed to check mounts: %w", err)
		}

		mounted := strings.Contains(mounts, "build_share on /mnt/build")
		if !mounted {
			fmt.Println("Mounting build_share...")
			if _, err := execAgent([]string{"mkdir", "-p", "/mnt/build"}, 30*time.Second, false); err != nil {
				return fmt.Errorf("mkdir /mnt/build failed: %w", err)
			}
			if _, err := execAgent([]string{"mount", "-t", "9p", "-o", "trans=virtio,version=9p2000.L,ro,msize=512000", "build_share", "/mnt/build"}, 30*time.Second, false); err != nil {
				return fmt.Errorf("mount build_share failed: %w", err)
			}
		}

		// List test binaries in host binDir
		files, err := os.ReadDir(binDir)
		if err != nil {
			return fmt.Errorf("failed to read bin dir %s: %w", binDir, err)
		}

		var failures int
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".test") {
				// Map host binDir to VM path structure relative to build dir
				absBinDir, _ := filepath.Abs(binDir)
				absBuildDir, _ := filepath.Abs(filepath.Join(projectRoot, "build"))

				relPath, err := filepath.Rel(absBuildDir, absBinDir)
				if err != nil || strings.HasPrefix(relPath, "..") {
					if idx := strings.Index(binDir, "tests/"); idx != -1 {
						relPath = binDir[idx:]
					} else {
						fmt.Printf("Warning: cannot map binDir %s to mount /mnt/build. Skipping %s.\n", binDir, f.Name())
						continue
					}
				}

				if filter != "" {
					if !strings.Contains(f.Name(), filter) {
						if !strings.HasPrefix(filter, "Test") {
							continue
						}
					}
				}

				vmPath := filepath.Join("/mnt/build", relPath, f.Name())
				fmt.Printf("Running Test: %s\n", f.Name())

				cmd := []string{vmPath}
				if len(finalTestArgs) > 0 {
					cmd = append(cmd, finalTestArgs...)
				} else {
					cmd = append(cmd, "-test.v")
				}

				// 10 minute timeout per test package
				if _, err := execAgent(cmd, 10*time.Minute, false); err != nil {
					fmt.Printf("FAIL: %s (%v)\n", f.Name(), err)
					failures++
				} else {
					fmt.Printf("PASS: %s\n", f.Name())
				}
			}
		}

		fmt.Println("----------------------------------------")
		if failures > 0 {
			return fmt.Errorf("%d test packages failed", failures)
		}
		fmt.Println("All Cross-Compiled Tests Passed.")
		return nil
	}

	// Fallback to source-based go test
	cmd := []string{"go", "test", "./..."}
	if len(testArgs) > 0 {
		cmd = append([]string{"go", "test"}, testArgs...)
	}

	fmt.Printf("Running: %s\n", strings.Join(cmd, " "))
	fmt.Println("----------------------------------------")

	if _, err := execAgent(cmd, 10*time.Minute, false); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	fmt.Println("----------------------------------------")
	fmt.Println("Tests Passed.")
	return nil
}
func formatDisplayName(name string, subtest string, target string) string {
	displayName := name
	if target != "" {
		prefix := fmt.Sprintf("integration_tests/%s/", target)
		displayName = strings.TrimPrefix(displayName, prefix)
	} else {
		displayName = strings.TrimPrefix(displayName, "integration_tests/linux/")
	}

	if subtest != "" && strings.HasSuffix(displayName, "/*") {
		// Batch job with active subtest: show the actual script path instead of wildcard
		subName := subtest
		if target != "" {
			prefix := fmt.Sprintf("integration_tests/%s/", target)
			subName = strings.TrimPrefix(subName, prefix)
		} else {
			subName = strings.TrimPrefix(subName, "integration_tests/linux/")
		}
		displayName = subName
	}

	// Truncate displayName if too long to maintain alignment
	if len(displayName) > 45 {
		displayName = "..." + displayName[len(displayName)-42:]
	}
	return displayName
}
