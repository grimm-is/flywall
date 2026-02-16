// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package agent

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"grimm.is/flywall/tools/pkg/protocol"
)

func handlePortOpen(msg protocol.Message, procs map[string]*ActiveProcess, mu *sync.Mutex, send func(protocol.Message) error) {
	// Parse payload
	payloadBytes, _ := json.Marshal(msg.Payload)
	var req protocol.PortOpenPayload
	json.Unmarshal(payloadBytes, &req)

	fmt.Fprintf(os.Stderr, "[Agent] Port Open Request: %s %s (Ref: %s)\n", req.Network, req.Address, msg.Ref)

	// Fetch the placeholder created by the main loop
	mu.Lock()
	proc, ok := procs[msg.Ref]
	mu.Unlock()
	if !ok || proc == nil {
		fmt.Fprintf(os.Stderr, "[Agent] Error: No placeholder for Ref %s\n", msg.Ref)
		return
	}
	dataChan := proc.DataChan

	conn, err := net.DialTimeout(req.Network, req.Address, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Agent] Port Dial Failed: %v\n", err)
		send(protocol.Message{Type: protocol.MsgError, Ref: msg.Ref, Error: fmt.Sprintf("dial failed: %v", err)})
		send(protocol.Message{Type: protocol.MsgPortClose, Ref: msg.Ref})

		mu.Lock()
		delete(procs, msg.Ref)
		close(dataChan)
		mu.Unlock()
		return
	}

	proc.Conn = conn
	fmt.Fprintf(os.Stderr, "[Agent] Port Connected: %s (Ref: %s)\n", req.Address, msg.Ref)

	// Start write loop (Protocol -> Conn)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for data := range dataChan {
			// fmt.Fprintf(os.Stderr, "[Agent] Writing %d bytes to conn (Ref: %s)\n", len(data), msg.Ref)
			if _, err := conn.Write(data); err != nil {
				fmt.Fprintf(os.Stderr, "[Agent] Port Write Error: %v\n", err)
				break
			}
		}
		conn.Close()
	}()

	// Start read loop (Conn -> Protocol)
	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			// fmt.Fprintf(os.Stderr, "[Agent] Read %d bytes from conn (Ref: %s)\n", n, msg.Ref)
			data := make([]byte, n)
			copy(data, buf[:n])
			if sendErr := send(protocol.Message{Type: protocol.MsgPortData, Ref: msg.Ref, Data: data}); sendErr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}

	// Cleanup
	mu.Lock()
	if _, ok := procs[msg.Ref]; ok {
		delete(procs, msg.Ref)
		close(dataChan)
	}
	mu.Unlock()

	conn.Close()
	<-done // Wait for write loop to finish flushing

	send(protocol.Message{Type: protocol.MsgPortClose, Ref: msg.Ref})
}

func handlePortData(msg protocol.Message, procs map[string]*ActiveProcess, mu *sync.Mutex) {
	mu.Lock()
	proc, ok := procs[msg.Ref]
	mu.Unlock()

	if ok && proc.DataChan != nil {
		// Use non-blocking send or check for closed channel?
		// Since we delete from procs before closing the channel,
		// and we hold the lock, we can minimize risks.
		// However, handlePortOpen might close it after we unlock but before we send.
		// In Go, sending to a closed channel panics.

		// Safer approach: handlePortOpen only closes after removing from map.
		// But there's still a tiny race.
		defer func() {
			if r := recover(); r != nil {
				// Ignored panic if channel closed
			}
		}()
		proc.DataChan <- msg.Data
	}
}

func handlePortClose(msg protocol.Message, procs map[string]*ActiveProcess, mu *sync.Mutex) {
	mu.Lock()
	proc, ok := procs[msg.Ref]
	if ok {
		delete(procs, msg.Ref)
		if proc.DataChan != nil {
			close(proc.DataChan)
		}
		if proc.Conn != nil {
			proc.Conn.Close()
		}
	}
	mu.Unlock()
}
