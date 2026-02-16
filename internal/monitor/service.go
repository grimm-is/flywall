// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package monitor

import (
	"fmt"
	"sync"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"

	probing "github.com/prometheus-community/pro-bing"
)

// Result holds the latest monitoring result for a target.
type Result struct {
	Target    string        `json:"target"`
	RouteName string        `json:"route_name"`
	IsUp      bool          `json:"is_up"`
	Latency   time.Duration `json:"latency"`
	LastCheck time.Time     `json:"last_check"`
	Error     string        `json:"error,omitempty"`
}

// Service manages background monitoring of routes.
type Service struct {
	logger     *logging.Logger
	routes     []config.Route
	results    map[string]*Result // Key: RouteName
	resultsMu  sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
	isTestMode bool
}

// NewService creates a new monitoring service.
func NewService(logger *logging.Logger, routes []config.Route) *Service {
	if logger == nil {
		logger = logging.New(logging.DefaultConfig())
	}
	return &Service{
		logger:  logger,
		routes:  routes,
		results: make(map[string]*Result),
		stopCh:  make(chan struct{}),
	}
}

// Start begins the monitoring loops.
func (s *Service) Start() {
	s.logger.Info("Starting monitoring service", "routes", len(s.routes))
	for _, r := range s.routes {
		if r.MonitorIP != "" {
			s.wg.Add(1)
			go s.monitorRoute(r)
		}
	}
}

// Stop stops all monitoring loops.
func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("Monitoring service stopped")
}

// GetResults returns the latest monitoring results.
func (s *Service) GetResults() []Result {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()

	results := make([]Result, 0, len(s.results))
	for _, res := range s.results {
		results = append(results, *res)
	}
	return results
}

// SetTestMode enables test mode (single check and exit).
func (s *Service) SetTestMode(enabled bool) {
	s.isTestMode = enabled
}

func (s *Service) monitorRoute(r config.Route) {
	defer s.wg.Done()

	s.logger.Debug("Starting monitoring", "route", r.Name, "target", r.MonitorIP)

	// Initial check
	s.check(r)

	if s.isTestMode {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.check(r)
		case <-s.stopCh:
			return
		}
	}
}

func (s *Service) check(r config.Route) {
	latency, err := checkPing(r.MonitorIP)

	s.resultsMu.Lock()
	res := &Result{
		Target:    r.MonitorIP,
		RouteName: r.Name,
		IsUp:      err == nil,
		Latency:   latency,
		LastCheck: time.Now(),
	}
	if err != nil {
		res.Error = err.Error()
		s.logger.Warn("Route is DOWN", "route", r.Name, "target", r.MonitorIP, "error", err)
	}
	s.results[r.Name] = res
	s.resultsMu.Unlock()
}

// Legacy Start function for backward compatibility during refactor
func Start(logger *logging.Logger, routes []config.Route, wg *sync.WaitGroup, isTestMode bool) {
	svc := NewService(logger, routes)
	svc.SetTestMode(isTestMode)
	svc.Start()
	if isTestMode {
		svc.wg.Wait()
	}
}

var CheckPingFunc = func(ip string) (time.Duration, error) {
	pinger, err := probing.NewPinger(ip)
	if err != nil {
		return 0, fmt.Errorf("failed to create pinger: %w", err)
	}

	pinger.Count = 1
	pinger.Timeout = 1 * time.Second
	pinger.SetPrivileged(false)

	err = pinger.Run()
	if err != nil {
		return 0, err
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return 0, fmt.Errorf("packet loss")
	}
	return stats.AvgRtt, nil
}

func checkPing(ip string) (time.Duration, error) {
	return CheckPingFunc(ip)
}