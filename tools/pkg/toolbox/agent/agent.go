// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"grimm.is/flywall/tools/pkg/protocol"
)

type ActiveProcess struct {
	Cmd      *exec.Cmd
	Stdin    io.WriteCloser
	Conn     net.Conn
	DataChan chan []byte
}

// Run is the entrypoint for the V2 agent
func Run(args []string) error {
	// Standardize working directory for relative test paths
	if stat, err := os.Stat("/mnt/flywall"); err == nil && stat.IsDir() {
		os.Chdir("/mnt/flywall")
	}

	port, err := openVirtioPort()
	if err != nil {
		return fmt.Errorf("failed to open serial port: %w", err)
	}
	defer port.Close()

	// Protocol Streams
	dec := json.NewDecoder(port)
	enc := json.NewEncoder(port)
	encMutex := &sync.Mutex{}

	// Sending helper
	send := func(msg protocol.Message) error {
		encMutex.Lock()
		defer encMutex.Unlock()
		return enc.Encode(msg)
	}

	// Active Processes
	procs := make(map[string]*ActiveProcess)
	procsMu := &sync.Mutex{}

	// Helper for stats
	getStats := func() protocol.HeartbeatPayload {
		free, load := getAgentStats()
		return protocol.HeartbeatPayload{FreeMemMB: free, LoadAvg: load}
	}

	// Ensure /dev/pts is mounted for PTY support
	if err := ensureDevPts(); err != nil {
		fmt.Fprintf(os.Stderr, "[Agent] Warning: failed to mount devpts: %v\n", err)
	}

	// Hello
	fmt.Fprintf(os.Stderr, "⚡ Agent starting: sending initial heartbeat\n")
	if err := send(protocol.Message{Type: protocol.MsgHeartbeat, Payload: getStats()}); err != nil {
		return err
	}

	// Periodic heartbeats
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			<-ticker.C
			send(protocol.Message{Type: protocol.MsgHeartbeat, Payload: getStats()})
		}
	}()

	// Main Loop
	for {
		var msg protocol.Message
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode error: %w", err)
		}

		switch msg.Type {
		case protocol.MsgExec:
			// fmt.Fprintf(os.Stderr, "[Agent] Dispatch: Exec ID=%s\n", msg.ID)
			go handleExec(msg, procs, procsMu, send)

		case protocol.MsgStdin:
			// fmt.Fprintf(os.Stderr, "[Agent] Dispatch: Stdin Ref=%s Len=%d\n", msg.Ref, len(msg.Data))
			handleStdin(msg, procs, procsMu)

		case protocol.MsgSignal:
			// fmt.Fprintf(os.Stderr, "[Agent] Dispatch: Signal Ref=%s\n", msg.Ref)
			handleSignal(msg, procs, procsMu)

		case protocol.MsgPortOpen:
			// fmt.Fprintf(os.Stderr, "[Agent] Dispatch: PortOpen Ref=%s\n", msg.Ref)
			// Synchronously register placeholder to avoid race with MsgPortData
			procsMu.Lock()
			procs[msg.Ref] = &ActiveProcess{DataChan: make(chan []byte, 256)}
			procsMu.Unlock()
			go handlePortOpen(msg, procs, procsMu, send)

		case protocol.MsgPortData:
			// fmt.Fprintf(os.Stderr, "[Agent] Dispatch: PortData Ref=%s Len=%d\n", msg.Ref, len(msg.Data))
			handlePortData(msg, procs, procsMu)

		case protocol.MsgPortClose:
			// fmt.Fprintf(os.Stderr, "[Agent] Dispatch: PortClose Ref=%s\n", msg.Ref)
			handlePortClose(msg, procs, procsMu)

		case protocol.MsgResize:
			handleResize(msg, procs, procsMu)
		}
	}
}

func handleResize(msg protocol.Message, procs map[string]*ActiveProcess, mu *sync.Mutex) {
	// Parse payload
	payloadBytes, _ := json.Marshal(msg.Payload)
	var req protocol.ResizePayload
	json.Unmarshal(payloadBytes, &req)

	mu.Lock()
	proc, ok := procs[msg.Ref]
	mu.Unlock()

	if ok && proc.Stdin != nil {
		if ptyFile, ok := proc.Stdin.(*os.File); ok {
			if err := pty.Setsize(ptyFile, &pty.Winsize{Rows: uint16(req.Rows), Cols: uint16(req.Cols)}); err != nil {
				fmt.Fprintf(os.Stderr, "[Agent] Resize failed: %v\n", err)
			}
		}
	}
}

// Reuse ActiveProcess map logic or create new one for ports?
// Can reuse 'procs' if we generalize ActiveProcess to hold net.Conn?
// Or just use a separate map. Separate map is cleaner.
// Actually, I'll extend ActiveProcess or make a separate map.
// To keep edits minimal, I'll add a 'ports' map to Run() scope or pass it around.
// But `handleExec` takes `procs`.
// Let's modify `Run` to hold `ports` map.

func handleExec(msg protocol.Message, procs map[string]*ActiveProcess, mu *sync.Mutex, send func(protocol.Message) error) {
	// Parse payload
	payloadBytes, _ := json.Marshal(msg.Payload)
	var req protocol.ExecPayload
	json.Unmarshal(payloadBytes, &req)

	if len(req.Command) == 0 {
		fmt.Fprintf(os.Stderr, "[Agent] Error: received empty command for job %s\n", msg.ID)
		send(protocol.Message{Type: protocol.MsgError, Ref: msg.ID, Error: "empty command"})
		return
	}

	cmd := exec.Command(req.Command[0], req.Command[1:]...)
	cmd.Dir = "/"
	if _, err := os.Stat("/mnt/flywall"); err == nil {
		cmd.Dir = "/mnt/flywall"
	}
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	cmd.Env = os.Environ()
	// Ensure standard PATH
	foundPath := false
	for i, env := range cmd.Env {
		if strings.HasPrefix(env, "PATH=") {
			cmd.Env[i] = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
			foundPath = true
			break
		}
	}
	if !foundPath {
		cmd.Env = append(cmd.Env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}

	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	fmt.Fprintf(os.Stderr, "[Agent] Exec: %v in %s (timeout: %ds)\n", cmd.Args, cmd.Dir, req.Timeout)

	// Create process group so we can kill all children
	// Only needed if NOT using PTY (PTY uses Setsid which implies new group)
	if !req.Tty {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Setup timeout killer if timeout is specified
	var timeoutCh chan struct{}
	var timedOut bool
	if req.Timeout > 0 {
		timeoutCh = make(chan struct{})
		go func() {
			timer := time.NewTimer(time.Duration(req.Timeout) * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				timedOut = true
				fmt.Fprintf(os.Stderr, "[Agent] Job %s TIMEOUT after %ds, killing process group\n", msg.ID, req.Timeout)
				// Kill entire process group (negative PID)
				if cmd.Process != nil {
					syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				}
			case <-timeoutCh:
				// Command completed before timeout
			}
		}()
	}

	var streamWg sync.WaitGroup
	var ptyFile *os.File

	// Shared sender for output
	sendOutput := func(t protocol.MessageType, data []byte) {
		send(protocol.Message{Type: t, Ref: msg.ID, Data: data})
	}

	isTty := req.Tty

	if isTty {
		var err error
		ptyFile, err = pty.Start(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Agent] PTY Start failed: %v\n", err)
			send(protocol.Message{Type: protocol.MsgError, Ref: msg.ID, Error: fmt.Sprintf("pty start: %v", err)})
			return
		}
		// NOTE: Do NOT defer ptyFile.Close() here, as it would close immediately
		// and send SIGHUP to the child. We close it in the Wait goroutine.

		proc := &ActiveProcess{Cmd: cmd, Stdin: ptyFile}
		mu.Lock()
		procs[msg.ID] = proc
		mu.Unlock()

		streamWg.Add(1)
		go func() {
			defer streamWg.Done()
			buf := make([]byte, 4096)
			for {
				n, err := ptyFile.Read(buf)
				if n > 0 {
					sendOutput(protocol.MsgStdout, buf[:n])
				}
				if err != nil {
					break
				}
			}
		}()
	} else {
		proc := &ActiveProcess{Cmd: cmd}
		mu.Lock()
		procs[msg.ID] = proc
		mu.Unlock()

		cmd.Stdout = &WriterProxy{Type: protocol.MsgStdout, Send: sendOutput}
		cmd.Stderr = &WriterProxy{Type: protocol.MsgStderr, Send: sendOutput}

		stdin, _ := cmd.StdinPipe()
		proc.Stdin = stdin

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[Agent] Start failed: %v\n", err)
			send(protocol.Message{Type: protocol.MsgError, Ref: msg.ID, Error: err.Error()})
			mu.Lock()
			delete(procs, msg.ID)
			mu.Unlock()
			return
		}
	}

	go func() {
		err := cmd.Wait()

		// Cancel timeout goroutine if it's running
		if timeoutCh != nil {
			close(timeoutCh)
		}

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		// Override exit code for timeout (killed by SIGKILL = -1 or 137)
		if timedOut {
			exitCode = 124 // Standard timeout exit code
			fmt.Fprintf(os.Stderr, "[Agent] Job %s killed due to timeout\n", msg.ID)
		} else {
			if exitCode == -1 {
				// Try to extract signal
				if exitErr, ok := err.(*exec.ExitError); ok {
					if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
						if status.Signaled() {
							fmt.Fprintf(os.Stderr, "[Agent] Job %s terminated by signal: %v\n", msg.ID, status.Signal())
						} else {
							fmt.Fprintf(os.Stderr, "[Agent] Job %s exited with -1 (Unknown cause)\n", msg.ID)
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "[Agent] Job %s exited with -1 (Error: %v)\n", msg.ID, err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "[Agent] Job %s exited with %d\n", msg.ID, exitCode)
			}
		}

		if isTty {
			// Close PTY master to ensure Read loop exits (if not already via EIO)
			ptyFile.Close()
			streamWg.Wait()
		}

		send(protocol.Message{Type: protocol.MsgExit, Ref: msg.ID, ExitCode: exitCode})

		mu.Lock()
		delete(procs, msg.ID)
		mu.Unlock()
	}()
}

type WriterProxy struct {
	Type protocol.MessageType
	Send func(protocol.MessageType, []byte)
}

func (w *WriterProxy) Write(p []byte) (n int, err error) {
	if len(p) > 0 {
		w.Send(w.Type, p)
	}
	return len(p), nil
}

func handleStdin(msg protocol.Message, procs map[string]*ActiveProcess, mu *sync.Mutex) {
	mu.Lock()
	proc, ok := procs[msg.Ref]
	mu.Unlock()

	if ok && proc.Stdin != nil {
		if len(msg.Data) == 0 {
			// Empty data usually means EOF/Close
			proc.Stdin.Close()
		} else {
			proc.Stdin.Write(msg.Data)
		}
	}
}

func handleSignal(msg protocol.Message, procs map[string]*ActiveProcess, mu *sync.Mutex) {
	mu.Lock()
	proc, ok := procs[msg.Ref]
	mu.Unlock()
	if ok && proc.Cmd.Process != nil {
		proc.Cmd.Process.Signal(syscall.Signal(msg.Signal))
	}
}

// Helpers from original code
func openVirtioPort() (*os.File, error) {
	paths := []string{
		"/dev/virtio-ports/flywall.agent",
		"/dev/vport0p1",
	}

	for i := 0; i < 10; i++ {
		for _, p := range paths {
			if f, err := os.OpenFile(p, os.O_RDWR, 0); err == nil {
				return f, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("no serial port found")
}

func getAgentStats() (int, float64) {
	freeMB := 0
	loadFn := 0.0

	// 1. Memory
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemAvailable:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					if kb, err := strconv.Atoi(parts[1]); err == nil {
						freeMB = kb / 1024
					}
				}
				break
			}
		}
	}

	// 2. Load
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			loadFn, _ = strconv.ParseFloat(parts[0], 64)
		}
	}

	return freeMB, loadFn
}
