package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"grimm.is/flywall/internal/api"
	"grimm.is/flywall/internal/auth"
	"grimm.is/flywall/internal/clock"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/kernel"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/state"
)

// Server encapsulates the simulator server logic.
type Server struct {
	apiServer *api.Server
	kernel    *kernel.SimKernel
	engine    *learning.Engine
	clock     *clock.MockClock
}

// StartServer initializes and starts the simulator server.
func StartServer(cfg *config.Config, k *kernel.SimKernel, e *learning.Engine, clk *clock.MockClock) error {
	// 1. Create SimControlPlaneClient
	simClient := NewSimControlPlaneClient(cfg, k, e)

	// 2. Setup Dependencies
	logger := logging.New(logging.Config{
		Output: os.Stdout,
		Level:  logging.LevelInfo,
	})
	authStore := auth.NewDevStore() // Auto-auth as dev admin

	stateOpts := state.DefaultOptions(":memory:")
	stateOpts.Clock = clk
	stateStore, err := state.NewSQLiteStore(stateOpts)
	if err != nil {
		return fmt.Errorf("failed to create state store: %w", err)
	}

	// 3. Setup API Server
	apiOpts := api.ServerOptions{
		Config:     cfg,
		Client:     simClient,
		AuthStore:  authStore,
		StateStore: stateStore,
		Logger:     logger,
		// LearningService: ... // Optional if we don't use it directly in API yet, or wrap engine
	}
	
	server, err := api.NewServer(apiOpts)
	if err != nil {
		return fmt.Errorf("failed to create API server: %w", err)
	}

	// 4. Wrap Handler to add Sim endpoints
	mux := http.NewServeMux()
	
	// API routes
	mux.Handle("/api/", server.Handler())

	// Sim routes
	mux.HandleFunc("/api/sim/replay", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			PCAPPath string `json:"pcap_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Starting replay of %s", req.PCAPPath)
		
		replayer := NewReplayer(k, e, clk)
		if err := replayer.Replay(req.PCAPPath); err != nil {
			log.Printf("Replay failed: %v", err)
			http.Error(w, fmt.Sprintf("Replay failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"stats": replayer.DiscoveryStats(),
		})
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// 5. Run Server
	log.Printf("Simulator Server listening on :8080 (MGMT interface)")
	
	// Use a channel to block until interrupt
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// SendReplayCommand sends a replay request to the running server.
func SendReplayCommand(pcapPath string) error {
	url := "http://localhost:8080/api/sim/replay"
	body := map[string]string{"pcap_path": pcapPath}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to contact server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(respBytes))
	}

	fmt.Println("Replay command sent successfully.")
	return nil
}
