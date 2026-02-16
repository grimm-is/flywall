// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tsnet

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
	"tailscale.com/tsnet"
)

// Server wraps the tsnet.Server
type Server struct {
	Config     *config.TsNetConfig
	Server     *tsnet.Server
	TargetAddr string // Address to proxy to (e.g., localhost:8080)
	StateDir   string
}

// NewServer creates a new TsNet server instance
func NewServer(cfg *config.TsNetConfig, stateDir string, targetAddr string) *Server {
	return &Server{
		Config:     cfg,
		StateDir:   stateDir,
		TargetAddr: targetAddr,
	}
}

// Start initializes and starts the tsnet server
func (s *Server) Start(ctx context.Context) error {
	hostname := s.Config.Hostname
	if hostname == "" {
		hostname = "flywall"
	}

	tsDir := filepath.Join(s.StateDir, "tsnet")
	if err := os.MkdirAll(tsDir, 0700); err != nil {
		return fmt.Errorf("failed to create tsnet state dir: %w", err)
	}

	s.Server = &tsnet.Server{
		Dir:       tsDir,
		Hostname:  hostname,
		AuthKey:   s.Config.AuthKey,
		Ephemeral: s.Config.Ephemeral,
		// Smart logging: Info for Auth URLs, Debug for everything else
		Logf: func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			// Promote Auth URLs to Info so they are visible without debug logs
			if strings.Contains(msg, "visit:") || strings.Contains(msg, "auth") && strings.Contains(msg, "http") {
				logging.Info("[tsnet] " + msg)
			} else {
				logging.Debug("[tsnet] " + msg)
			}
		},
	}

	// Custom Auth handling to print URL
	// tsnet prints to stdout by default if no callback provided, effectively.
	// We can hook output if we want to capture the URL.
	// But tsnet.Server doesn't have a direct "OnLoginURL" callback easy to hook without parsing logs?
	// Actually, tsnet uses the standard tailscale log structure.
	// Ideally we want to print the Auth URL to stdout clearly.

	// Start the listener
	ln, err := s.Server.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("tsnet listen error: %w", err)
	}

	// Also listen on 443 if you want TLS (tsnet handles ACME automatically)
	ln443, err := s.Server.Listen("tcp", ":443")
	if err != nil {
		return fmt.Errorf("tsnet listen 443 error: %w", err)
	}

	// Create a reverse proxy to the local API
	targetURL, err := url.Parse("http://" + s.TargetAddr)
	if err != nil {
		return fmt.Errorf("invalid target addr: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize the proxy director to handle host headers if needed
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Set X-Forwarded-For, etc?
		// Is this needed for local API?
		// Maybe set Host to match target?
		req.Host = targetURL.Host
	}

	// Handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.Debug("TsNet Request: %s %s", r.Method, r.URL.String())
		proxy.ServeHTTP(w, r)
	})

	// Start Servers
	go http.Serve(ln, handler)
	go http.Serve(ln443, handler)

	logging.Info(fmt.Sprintf("TsNet Server started on %s (proxying to %s)", hostname, s.TargetAddr))

	<-ctx.Done()
	return s.Server.Close()
}
