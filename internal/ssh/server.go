// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package ssh

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/logging"
	"grimm.is/flywall/internal/auth"
	"grimm.is/flywall/internal/config"
	fwlog "grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/metrics"
)

// Server wraps the Wish SSH server
type Server struct {
	srv       *ssh.Server
	config    *config.Config
	authStore *auth.Store
	collector *metrics.Collector
	addr      string

	// Internal counters
	activeSessions   int32
	totalConnections uint64
	authFailures     uint64
}

// NewServer creates a new SSH server
func NewServer(cfg *config.Config, authStore *auth.Store) (*Server, error) {
	if cfg.SSH == nil {
		return nil, fmt.Errorf("ssh configuration is nil")
	}

	addr := fmt.Sprintf("%s:%d", cfg.SSH.ListenAddress, cfg.SSH.Port)
	if addr == ":0" {
		addr = ":2222" // Default
	}

	// Create request-scoped logger adapter
	// We want to route Wish logs to our internal logging package
	// Wish 1.4.x uses MiddlewareWithLogger
	loggerMiddleware := logging.MiddlewareWithLogger(newAdapter())

	// Password Auth Handler
	passAuth := func(ctx ssh.Context, password string) bool {
		if authStore == nil {
			fwlog.Error("SSH: authStore is nil, denying access")
			return false
		}
		user := ctx.User()
		_, err := authStore.Authenticate(user, password)
		if err != nil {
			fwlog.Warn(fmt.Sprintf("SSH: Auth failed for user %s: %v", user, err))
			return false
		}
		fwlog.Info(fmt.Sprintf("SSH: Authenticated user %s", user))
		return true
	}

	srv := &Server{
		config:    cfg,
		authStore: authStore,
		addr:      addr,
	}

	ws, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(cfg.SSH.HostKeyPath),
		wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
			success := passAuth(ctx, password)
			if !success {
				atomic.AddUint64(&srv.authFailures, 1)
				srv.updateMetrics()
			}
			return success
		}),
		wish.WithMiddleware(
			loggerMiddleware,
			srv.measureMiddleware(),
		),
	)
	if err != nil {
		return nil, err
	}

	srv.srv = ws
	return srv, nil
}

// Start starts the SSH server
func (s *Server) Start(ctx context.Context) error {
	fwlog.Info("Starting SSH server on " + s.addr)

	// Create a listener
	// implementation note: wish.Server.ListenAndServe() creates its own listener.
	// But we want to respect context for shutdown.
	// We can use srv.Serve(l)

	// However, Start() is typically async in our service model.
	// The caller likely expects this to return nil and run in background?
	// Or block?
	// Our other services (tsnet) block or have Start() run non-blocking?
	// TsNet.Start() is blocking in runTsNet but the Service interface usually implies management.

	// Let's run it in a goroutine and return.
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			fwlog.Error(fmt.Sprintf("SSH server error: %v", err))
		}
	}()

	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	fwlog.Info("Stopping SSH server...")
	return s.srv.Shutdown(ctx)
}

func (s *Server) SetMetricsCollector(collector *metrics.Collector) {
	s.collector = collector
}

func (s *Server) updateMetrics() {
	if s.collector != nil {
		s.collector.UpdateSSHStats(
			true,
			int(atomic.LoadInt32(&s.activeSessions)),
			atomic.LoadUint64(&s.totalConnections),
			atomic.LoadUint64(&s.authFailures),
		)
	}
}

func (s *Server) measureMiddleware() wish.Middleware {
	return func(sh ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			atomic.AddInt32(&s.activeSessions, 1)
			atomic.AddUint64(&s.totalConnections, 1)
			s.updateMetrics()

			defer func() {
				atomic.AddInt32(&s.activeSessions, -1)
				s.updateMetrics()
			}()

			sh(sess)
		}
	}
}

// adapter adapts flywall logging to wish logging interface
type adapter struct{}

func newAdapter() *adapter {
	return &adapter{}
}

func (a *adapter) Printf(format string, args ...interface{}) {
	// Downgrade generic SSH logs to Debug to reduce spam
	fwlog.Debug(fmt.Sprintf("[ssh] "+format, args...))
}

func (a *adapter) Write(p []byte) (n int, err error) {
	msg := string(p)
	fwlog.Debug("[ssh] " + msg)
	return len(p), nil
}
