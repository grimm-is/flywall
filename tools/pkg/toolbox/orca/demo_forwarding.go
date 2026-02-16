// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package orca

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"grimm.is/flywall/tools/pkg/protocol"
	"grimm.is/flywall/tools/pkg/toolbox/orca/client"
	"grimm.is/flywall/tools/pkg/toolbox/vmm"
)

// DemoPortForwarder manages the multiplexed connection to the agent
// and the control plane for demo exec/shell commands.
type DemoPortForwarder struct {
	vm        *vmm.VM
	agentConn net.Conn
	enc       *json.Encoder
	dec       *json.Decoder
	encMu     sync.Mutex

	// Port Forwarding
	listeners []net.Listener
	conns     map[string]net.Conn // Ref -> Host Conn
	connsMu   sync.Mutex

	// Control Plane
	controlL  net.Listener
	clients   map[string]net.Conn // Ref -> CLI Conn
	clientsMu sync.Mutex

	running   bool
	ready     chan struct{}
	readyOnce sync.Once

	out io.Writer

	// Config
	httpPort  int
	httpsPort int
}

func startDemoPortForwarding(vm *vmm.VM, out io.Writer, httpPort, httpsPort int) *DemoPortForwarder {
	pf := &DemoPortForwarder{
		vm:        vm,
		conns:     make(map[string]net.Conn),
		clients:   make(map[string]net.Conn),
		running:   true,
		ready:     make(chan struct{}),
		out:       out,
		httpPort:  httpPort,
		httpsPort: httpsPort,
	}
	go pf.run()
	return pf
}

func (pf *DemoPortForwarder) WaitReady() {
	<-pf.ready
}

func (pf *DemoPortForwarder) run() {
	fmt.Fprintf(pf.out, "[Forwarder] Starting Control Server...\n")
	// Start Control Server
	if err := pf.startControlServer(); err != nil {
		fmt.Fprintf(pf.out, "[Forwarder] Failed to start control server: %v\n", err)
	}

	// 1. Connect to Agent
	fmt.Fprintf(pf.out, "[Forwarder] Waiting for Agent at socket: %s\n", pf.vm.SocketPath)
	for pf.running {
		conn, err := net.Dial("unix", pf.vm.SocketPath)
		if err == nil {
			pf.agentConn = conn
			pf.enc = json.NewEncoder(conn)
			pf.dec = json.NewDecoder(conn)
			fmt.Fprintf(pf.out, "[Forwarder] Connected to Agent! Starting session...\n")
			pf.readyOnce.Do(func() { close(pf.ready) })
			pf.handleSession()

			// Clear encoder on disconnect
			pf.encMu.Lock()
			pf.enc = nil
			pf.encMu.Unlock()

			if !pf.running {
				fmt.Fprintf(pf.out, "[Forwarder] Stopping (running=false)\n")
				return
			}
			fmt.Fprintf(pf.out, "[Forwarder] Connection lost, retrying...\n")
			time.Sleep(1 * time.Second)
		} else {
			// fmt.Printf("[Forwarder] Dial failed: %v\n", err) // Too noisy usually, but helpful if stuck
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (pf *DemoPortForwarder) handleSession() {
	fmt.Fprintf(pf.out, "[Forwarder] Starting Listeners on :%d and :%d...\n", pf.httpPort, pf.httpsPort)
	// Start Listeners
	go pf.startListener("tcp", fmt.Sprintf(":%d", pf.httpPort), "127.0.0.1:9090")
	go pf.startListener("tcp", fmt.Sprintf(":%d", pf.httpsPort), "127.0.0.1:8443")

	// Read Loop
	for {
		var msg protocol.Message
		if err := pf.dec.Decode(&msg); err != nil {
			fmt.Fprintf(pf.out, "[Forwarder] Decode error: %v\n", err)
			return
		}

		// fmt.Printf("[Forwarder] Recv: %s Ref=%s DataLen=%d\n", msg.Type, msg.Ref, len(msg.Data))

		switch msg.Type {
		case protocol.MsgHeartbeat:
			// fmt.Printf("[Forwarder] ❤️ Heartbeat\n")
		case protocol.MsgPortData:
			// fmt.Printf("[Forwarder] Recv Port Data: Ref=%s Len=%d\n", msg.Ref, len(msg.Data))
		}

		switch msg.Type {
		case protocol.MsgPortData:
			pf.connsMu.Lock()
			conn, ok := pf.conns[msg.Ref]
			pf.connsMu.Unlock()
			if ok {
				conn.Write(msg.Data)
			}
		case protocol.MsgPortClose:
			pf.closeConn(msg.Ref)

		// Exec/Shell Output Routing
		case protocol.MsgStdout, protocol.MsgStderr, protocol.MsgExit, protocol.MsgError:
			// fmt.Printf("[Forwarder] Routing Exec Output %s\n", msg.Type)
			pf.clientsMu.Lock()
			client, ok := pf.clients[msg.Ref]
			pf.clientsMu.Unlock()
			if ok {
				// Forward JSON message to client
				if err := json.NewEncoder(client).Encode(msg); err != nil {
					// Client dead?
					pf.closeClient(msg.Ref)
				}
				if msg.Type == protocol.MsgExit {
					pf.closeClient(msg.Ref)
				}
			}
		}
	}
}

// --- Port Forwarding ---

func (pf *DemoPortForwarder) startListener(netw, addr, targetAddr string) {
	l, err := net.Listen(netw, addr)
	if err != nil {
		fmt.Fprintf(pf.out, "[Forwarder] Failed to listen on %s: %v\n", addr, err)
		return
	}
	pf.listeners = append(pf.listeners, l)
	fmt.Fprintf(pf.out, "[Forwarder] Listening on %s (Relay to %s)\n", addr, targetAddr)

	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		fmt.Fprintf(pf.out, "[Forwarder] [%s] Accepted connection from %s\n", addr, conn.RemoteAddr())
		go pf.handleHostConn(conn, netw, targetAddr)
	}
}

func (pf *DemoPortForwarder) handleHostConn(conn net.Conn, netw, targetAddr string) {
	id := uuid.New().String()

	pf.connsMu.Lock()
	pf.conns[id] = conn
	pf.connsMu.Unlock()

	// Send Open Request
	payload := protocol.PortOpenPayload{
		Network: netw,
		Address: targetAddr,
	}
	pf.send(protocol.Message{
		Type:    protocol.MsgPortOpen,
		Ref:     id,
		Payload: payload,
	})

	// Read Loop
	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			if err := pf.send(protocol.Message{Type: protocol.MsgPortData, Ref: id, Data: buf[:n]}); err != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}

	pf.closeConn(id)
	pf.send(protocol.Message{Type: protocol.MsgPortClose, Ref: id})
}

func (pf *DemoPortForwarder) closeConn(id string) {
	pf.connsMu.Lock()
	conn, ok := pf.conns[id]
	if ok {
		conn.Close()
		delete(pf.conns, id)
	}
	pf.connsMu.Unlock()
}

// --- Control Plane ---

func (pf *DemoPortForwarder) startControlServer() error {
	socketPath := filepath.Join(pf.vm.Config.BuildDir, "flywall-demo.sock")
	os.Remove(socketPath)
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	pf.controlL = l

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go pf.handleControlClient(conn)
		}
	}()
	return nil
}

func (pf *DemoPortForwarder) handleControlClient(conn net.Conn) {
	dec := json.NewDecoder(conn)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			conn.Close()
			return
		}

		// 1. Try protocol.Message (stdin)
		var msg protocol.Message
		if err := json.Unmarshal(raw, &msg); err == nil && msg.Type == protocol.MsgStdin {
			pf.send(msg)
			continue
		}

		// 2. Try ClientRequest (submit)
		var req protocol.ClientRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}

		// Handle Exec request
		if req.Type == "exec" || req.Type == "shell" {
			job := req.Job
			if job.ID == "" {
				job.ID = uuid.New().String()
			}

			// Register Client
			pf.clientsMu.Lock()
			pf.clients[job.ID] = conn
			pf.clientsMu.Unlock()

			// 2. Prepare Exec Payload
			// We prefer top-level fields (from RunExecWithSocket) then fallback to Job fields
			command := req.Command
			if len(command) == 0 {
				command = job.Command
			}
			env := req.Env
			if len(env) == 0 {
				env = job.Env
			}
			tty := req.Tty
			if !tty {
				tty = job.Tty
			}

			execReq := protocol.ExecPayload{
				Command: command,
				Env:     env,
				Tty:     tty,
				Timeout: int(job.Timeout.Seconds()),
			}
			// If shell, override
			if req.Type == "shell" {
				execReq.Command = []string{"/bin/sh"}
				execReq.Tty = true
			}

			// Send to Agent
			if err := pf.send(protocol.Message{
				Type:    protocol.MsgExec,
				ID:      job.ID,
				Payload: execReq,
			}); err != nil {
				// Agent unreachable, report error to client
				json.NewEncoder(conn).Encode(protocol.Message{
					Type:  protocol.MsgError,
					Ref:   job.ID,
					Error: fmt.Sprintf("failed to send to agent: %v", err),
				})
				conn.Close() // Close client conn
			}
		}
	}
}

func (pf *DemoPortForwarder) closeClient(id string) {
	pf.clientsMu.Lock()
	// We don't close the connection usually as it might be persistent/multiplexed?
	// But in our simple runDemoExec, one conn = one job.
	// So we can remove it.
	delete(pf.clients, id)
	pf.clientsMu.Unlock()
}

func (pf *DemoPortForwarder) send(msg protocol.Message) error {
	pf.encMu.Lock()
	defer pf.encMu.Unlock()
	if pf.enc == nil {
		return fmt.Errorf("not connected")
	}
	// fmt.Printf("[Forwarder] Sending %s Ref=%s\n", msg.Type, msg.Ref)
	return pf.enc.Encode(msg)
}

// --- Client Implementation ---

func runDemoExec(cmd []string, shell bool) error {
	_, buildDir := locateBuildDir()
	socketPath := filepath.Join(buildDir, "flywall-demo.sock")

	if shell && len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}

	return client.RunExecWithSocket(cmd, shell || (len(cmd) == 0), "", socketPath, os.Stdout, os.Stderr)
}
