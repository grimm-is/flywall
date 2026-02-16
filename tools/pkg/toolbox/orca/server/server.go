// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/tools/pkg/protocol"
	"grimm.is/flywall/tools/pkg/toolbox/vmm"
)

// Server manages the lifecycle of VMs and routes messages
type Server struct {
	config vmm.Config

	// VM Pool
	vms   map[string]*VMInst
	vmsMu sync.RWMutex

	// Scheduler
	jobQueue chan jobRequest

	// Message Routing
	routes   map[string]route
	routesMu sync.Mutex

	listener    net.Listener
	shutdown    chan struct{}
	cleanupDone chan struct{}

	warmSize int
	maxSize  int
}

type route struct {
	conn net.Conn
	vm   *VMInst
	done func()
}

type VMInst struct {
	ID         string
	Conn       net.Conn
	VM         *vmm.VM
	Status     string
	LastHealth time.Time
	Busy       bool
	ActiveJobs int
	LastJob    string
	JobHistory []string
	FreeMemMB  int
	LoadAvg    float64
}

type jobRequest struct {
	Job      protocol.Job
	TargetVM string
	Client   net.Conn
}

func New(cfg vmm.Config, warm, max int) *Server {
	s := &Server{
		config:      cfg,
		vms:         make(map[string]*VMInst),
		jobQueue:    make(chan jobRequest, 1000),
		routes:      make(map[string]route),
		shutdown:    make(chan struct{}),
		cleanupDone: make(chan struct{}),
		warmSize:    warm,
		maxSize:     max,
	}
	go s.scheduler()
	go s.healthChecker()
	return s
}

func (s *Server) healthChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.vmsMu.Lock()
			for _, vm := range s.vms {
				if vm.Status == "ready" || vm.Status == "connected" {
					if time.Since(vm.LastHealth) > 60*time.Second {
						vm.Status = "stale"
						fmt.Printf("Worker %s: Marked stale (no heartbeat in 60s)\n", vm.ID)
					}
				}
			}
			s.vmsMu.Unlock()
		case <-s.shutdown:
			return
		}
	}
}

func (s *Server) Start(socketPath, pidPath string) error {
	_ = os.Remove(socketPath)
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	s.listener = l

	if pidPath != "" {
		if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
			l.Close()
			return fmt.Errorf("failed to write pid file: %w", err)
		}
	}

	go s.acceptLoop()

	go func() {
		<-s.shutdown
		s.vmsMu.Lock()
		for _, vm := range s.vms {
			if vm.Conn != nil {
				vm.Conn.Close()
			}
			if vm.VM != nil {
				vm.VM.Stop()
			}
		}
		s.vmsMu.Unlock()
		if s.listener != nil {
			s.listener.Close()
		}
		_ = os.Remove(socketPath) // Clean up server socket file
		if pidPath != "" {
			_ = os.Remove(pidPath)
		}
		close(s.cleanupDone)
	}()

	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleClient(conn)
	}
}

// Done returns a channel that is closed when the server shuts down
func (s *Server) Done() <-chan struct{} {
	return s.shutdown
}

// Stop initiates shutdown and waits for cleanup
func (s *Server) Stop() {
	select {
	case <-s.shutdown:
		// Already shutting down
	default:
		close(s.shutdown)
	}
	<-s.cleanupDone
}

func (s *Server) scheduler() {
	for {
		var req jobRequest
		select {
		case req = <-s.jobQueue:
			fmt.Printf("Scheduler: Received job %s\n", req.Job.ID)
		case <-s.shutdown:
			fmt.Println("Scheduler: Shutting down.")
			return
		}

		// Find idle worker
		var worker *VMInst
		iterations := 0
		targetNotFound := false
		for {
			s.vmsMu.Lock()
			count := len(s.vms)
			ready := 0

			if req.TargetVM != "" {
				// Target specific VM - allow bypass if connected (multi-tenant)
				if v, ok := s.vms[req.TargetVM]; ok {
					if v.Status == "connected" || v.Status == "ready" {
						v.ActiveJobs++
						v.Busy = true
						worker = v
					}
				} else if iterations == 0 {
					// VM doesn't exist at all - fail immediately
					targetNotFound = true
				}
			} else {
				// Find any idle worker
				for _, v := range s.vms {
					if (v.Status == "connected" || v.Status == "ready") && !v.Busy {
						v.ActiveJobs++
						v.Busy = true
						worker = v
						break
					}
					if v.Status == "ready" {
						ready++
					}
				}
			}
			s.vmsMu.Unlock()

			if targetNotFound {
				fmt.Printf("Scheduler: Target VM %s not found, failing job %s\n", req.TargetVM, req.Job.ID)
				if req.Client != nil {
					errMsg := protocol.Message{
						Type:  protocol.MsgError,
						Ref:   req.Job.ID,
						Error: fmt.Sprintf("VM %s not found", req.TargetVM),
					}
					json.NewEncoder(req.Client).Encode(errMsg)
				}
				break
			}

			if worker != nil {
				break
			}

			// Try to scale up if below maxSize
			s.vmsMu.Lock()
			if len(s.vms) < s.maxSize {
				nextID := len(s.vms) + 1
				// Check if this ID is already taken (due to deletions)
				for {
					idStr := fmt.Sprintf("%d", nextID)
					if _, exists := s.vms[idStr]; !exists {
						break
					}
					nextID++
				}
				s.vmsMu.Unlock()
				fmt.Printf("Scheduler: Scaling up! Starting VM %d (Current: %d/%d)\n", nextID, count, s.maxSize)
				if err := s.StartVM(nextID); err != nil {
					fmt.Printf("Scheduler: Failed to scale up: %v\n", err)
				}
			} else {
				s.vmsMu.Unlock()
			}

			if iterations%50 == 0 && iterations > 0 { // Every 10s
				fmt.Printf("Scheduler: Waiting for worker (Total: %d, Ready: %d)\n", count, ready)
			}
			iterations++
			time.Sleep(200 * time.Millisecond) // Wait for worker
		}

		if worker == nil {
			continue // Skip dispatch if no worker found (target not found case)
		}

		fmt.Printf("Scheduler: Dispatching job %s to %s\n", req.Job.ID, worker.ID)
		// Dispatch
		go s.runJob(worker, req)
	}
}

func (s *Server) runJob(vm *VMInst, req jobRequest) {
	fmt.Printf("runJob: VM %s executing job %s\n", vm.ID, req.Job.ID)

	s.vmsMu.Lock()
	vm.LastJob = req.Job.ScriptPath
	vm.JobHistory = append(vm.JobHistory, req.Job.ScriptPath)
	s.vmsMu.Unlock()

	doneCh := make(chan struct{})

	// Register Route with Done signal and VM reference
	s.routesMu.Lock()
	s.routes[req.Job.ID] = route{
		conn: req.Client,
		vm:   vm,
		done: func() { close(doneCh) },
	}
	s.routesMu.Unlock()

	// Cleanup
	defer func() {
		s.routesMu.Lock()
		delete(s.routes, req.Job.ID)
		s.routesMu.Unlock()

		s.vmsMu.Lock()
		vm.ActiveJobs--
		if vm.ActiveJobs <= 0 {
			vm.ActiveJobs = 0
			vm.Busy = false
		}
		s.vmsMu.Unlock()
		fmt.Printf("runJob: Job %s completed on VM %s\n", req.Job.ID, vm.ID)
	}()

	// Check for batch mode
	if len(req.Job.Scripts) > 0 {
		// Batch mode: run all scripts sequentially
		s.runBatchJob(vm, req, doneCh)
		return
	}

	// Single script mode
	cmd := req.Job.Command
	if len(cmd) == 0 && req.Job.ScriptPath != "" {
		cmd = []string{"/bin/sh", req.Job.ScriptPath}
	}

	execReq := protocol.ExecPayload{
		Command: cmd,
		Env:     req.Job.Env,
		Tty:     req.Job.Tty,
		Timeout: int(req.Job.Timeout.Seconds()),
	}

	msg := protocol.Message{
		Type:    protocol.MsgExec,
		ID:      req.Job.ID,
		Payload: execReq,
	}

	fmt.Printf("runJob: Sending exec request for job %s to VM %s\n", req.Job.ID, vm.ID)
	if s.config.Trace {
		raw, _ := json.Marshal(msg)
		fmt.Printf("TRACE: [Srv -> %s] %s\n", vm.ID, string(raw))
	}
	if err := json.NewEncoder(vm.Conn).Encode(msg); err != nil {
		fmt.Printf("runJob: Failed to send exec request for job %s to VM %s: %v\n", req.Job.ID, vm.ID, err)
		return
	}

	// Wait for completion (MsgExit)
	<-doneCh
}

// runBatchJob executes multiple scripts sequentially in the same VM.
// It generates a wrapper script that runs all tests and outputs TAP14 subtests.
func (s *Server) runBatchJob(vm *VMInst, req jobRequest, finalDone chan struct{}) {
	scripts := req.Job.Scripts
	total := len(scripts)

	fmt.Printf("runBatchJob: Running %d scripts in batch on VM %s\n", total, vm.ID)

	// Build a shell command that runs all scripts sequentially with TAP14 formatting
	// The command outputs TAP14 with each script as a subtest
	var cmd strings.Builder
	cmd.WriteString("set -e; echo 'TAP version 14'; echo '1..")
	cmd.WriteString(fmt.Sprintf("%d", total))
	cmd.WriteString("'; passed=0; failed=0; ")

	for i, script := range scripts {
		subNum := i + 1
		// Each script: run, capture exit code, output ok/not ok
		cmd.WriteString(fmt.Sprintf(
			"echo '# Subtest: %s'; if /bin/sh /mnt/flywall/%s 2>&1; then echo 'ok %d - %s'; passed=$((passed+1)); else echo 'not ok %d - %s'; failed=$((failed+1)); fi; ",
			script, script, subNum, script, subNum, script,
		))
	}
	cmd.WriteString("[ $failed -eq 0 ]") // Exit 0 if all passed

	execReq := protocol.ExecPayload{
		Command: []string{"/bin/sh", "-c", cmd.String()},
		Env:     req.Job.Env,
		Timeout: int(req.Job.Timeout.Seconds()),
	}

	msg := protocol.Message{
		Type:    protocol.MsgExec,
		ID:      req.Job.ID,
		Payload: execReq,
	}

	fmt.Printf("runBatchJob: Sending batch exec to VM %s\n", vm.ID)
	if err := json.NewEncoder(vm.Conn).Encode(msg); err != nil {
		fmt.Printf("runBatchJob: Failed to send exec: %v\n", err)
		return
	}

	// Wait for completion (existing route handling will signal doneCh via r.done())
	<-finalDone
}

func (s *Server) StartVM(id int) error {
	vmCfg := s.config
	vm, err := vmm.NewVM(vmCfg, id)
	if err != nil {
		return err
	}

	vmID := fmt.Sprintf("%d", id)
	inst := &VMInst{
		ID:     vmID,
		VM:     vm,
		Status: "starting",
	}

	s.vmsMu.Lock()
	s.vms[vmID] = inst
	s.vmsMu.Unlock()

	go func() {
		fmt.Printf("VM %s starting...\n", vmID)
		err := vm.Start(context.Background())
		fmt.Printf("VM %s exited: %v\n", vmID, err)

		// Ensure cleanup happens even if it crashed/exited unexpectedly
		vm.Stop()

		s.vmsMu.Lock()
		delete(s.vms, vmID)
		s.vmsMu.Unlock()
	}()

	go s.connectAgent(inst, vm.SocketPath)
	return nil
}

func (s *Server) connectAgent(inst *VMInst, socketPath string) {
	fmt.Printf("Worker %s: Connecting to Agent at %s\n", inst.ID, socketPath)
	for i := 0; i < 600; i++ { // Increase to 60s for slow boot
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			fmt.Printf("Worker %s: Connected!\n", inst.ID)
			inst.Conn = conn
			inst.LastHealth = time.Now()
			inst.Status = "connected"
			go s.handleAgent(inst)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Printf("Worker %s: Connection FAILED after 60s\n", inst.ID)
	inst.Status = "failed"
}

func (s *Server) handleAgent(inst *VMInst) {
	dec := json.NewDecoder(inst.Conn)
	for {
		var msg protocol.Message
		if err := dec.Decode(&msg); err != nil {
			break
		}

		// Inject Worker ID
		msg.WorkerID = inst.ID

		if msg.Ref != "" {
			if s.config.Trace {
				raw, _ := json.Marshal(msg)
				fmt.Printf("TRACE: [%s -> Srv] %s\n", inst.ID, string(raw))
			}
			s.routesMu.Lock()
			r, ok := s.routes[msg.Ref]
			s.routesMu.Unlock()

			if ok {
				if s.config.Trace {
					raw, _ := json.Marshal(msg)
					fmt.Printf("TRACE: [Srv -> Client] %s\n", string(raw))
				}
				json.NewEncoder(r.conn).Encode(msg)
				if msg.Type == protocol.MsgExit {
					if r.done != nil {
						r.done()
					}
				}
			}
		} else if msg.Type == protocol.MsgHeartbeat {
			inst.LastHealth = time.Now()
			inst.Status = "ready"

			// Parse stats
			if msg.Payload != nil {
				data, _ := json.Marshal(msg.Payload)
				var hb protocol.HeartbeatPayload
				if err := json.Unmarshal(data, &hb); err == nil {
					inst.FreeMemMB = hb.FreeMemMB
					inst.LoadAvg = hb.LoadAvg
				}
			}
		}
	}
	inst.Status = "disconnected"
}

func (s *Server) handleClient(conn net.Conn) {
	fmt.Printf("Client connected\n")
	defer conn.Close()
	dec := json.NewDecoder(conn)

	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			fmt.Printf("Client disconnected\n")
			return
		}

		// 1. Try protocol.Message (for interactive stdin)
		var msg protocol.Message
		if err := json.Unmarshal(raw, &msg); err == nil && msg.Type == protocol.MsgStdin {
			s.routesMu.Lock()
			r, ok := s.routes[msg.Ref]
			s.routesMu.Unlock()
			if ok && r.vm != nil && r.vm.Conn != nil {
				r.vm.Conn.Write(append(raw, '\n'))
			}
			continue
		}

		// 2. Try ClientRequest
		var req protocol.ClientRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}

		switch req.Type {
		case "submit_job":
			fmt.Printf("Job submitted: %s (%s)\n", req.Job.ID, req.Job.ScriptPath)
			s.jobQueue <- jobRequest{Job: req.Job, Client: conn, TargetVM: req.TargetVM}
		case "exec", "shell":
			job := req.Job
			if req.Type == "shell" {
				job.Command = []string{"/bin/sh"}
				job.Tty = true
			} else if len(req.Command) > 0 {
				job.Command = req.Command
				job.Tty = req.Tty
			}
			if job.ID == "" {
				job.ID = fmt.Sprintf("exec-%d", time.Now().UnixNano())
			}
			fmt.Printf("Interactive %s submitted: %s (target: %s)\n", req.Type, job.ID, req.TargetVM)
			s.jobQueue <- jobRequest{Job: job, Client: conn, TargetVM: req.TargetVM}
		case "status":
			s.vmsMu.Lock()
			resp := protocol.StatusResponse{
				VMs:      make([]protocol.VMInfo, 0, len(s.vms)),
				WarmSize: s.warmSize,
				MaxSize:  s.maxSize,
			}
			for _, v := range s.vms {
				resp.VMs = append(resp.VMs, protocol.VMInfo{
					ID:         v.ID,
					Status:     v.Status,
					Busy:       v.Busy,
					ActiveJobs: v.ActiveJobs,
					LastHealth: fmt.Sprintf("%s ago", time.Since(v.LastHealth).Round(time.Second)),
					LastJob:    v.LastJob,
					JobHistory: v.JobHistory,
					FreeMemMB:  v.FreeMemMB,
					LoadAvg:    v.LoadAvg,
				})
			}
			s.vmsMu.Unlock()
			json.NewEncoder(conn).Encode(resp)
		case "shutdown":
			close(s.shutdown)
			return
		}
	}
}
