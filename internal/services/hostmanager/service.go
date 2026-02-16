// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package hostmanager

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// Service manages dynamic hostname resolution and IPSet updates.
type Service struct {
	cfg    *config.Config
	logger *logging.Logger
	stop   chan struct{}
	wg     sync.WaitGroup
}

// New creates a new HostManager service.
func New(cfg *config.Config, logger *logging.Logger) *Service {
	return &Service{
		cfg:    cfg,
		logger: logger,
		stop:   make(chan struct{}),
	}
}

// Start begins the background resolution tasks.
func (s *Service) Start() error {
	s.logger.Info("Starting HostManager service")

	found := false
	for _, ipset := range s.cfg.IPSets {
		if ipset.Type == "dns" && len(ipset.Domains) > 0 {
			found = true
			s.wg.Add(1)
			go s.manageSet(ipset)
		}
	}

	if !found {
		s.logger.Info("No DNS-based IPSets configured")
	}

	return nil
}

// Stop stops the service.
func (s *Service) Stop() error {
	s.logger.Info("Stopping HostManager service")
	close(s.stop)
	s.wg.Wait()
	return nil
}

func (s *Service) manageSet(ipset config.IPSet) {
	defer s.wg.Done()

	interval := 5 * time.Minute
	if ipset.RefreshInterval != "" {
		if d, err := time.ParseDuration(ipset.RefreshInterval); err == nil {
			interval = d
		}
	}

	s.logger.Info("Managing DNS set", "name", ipset.Name, "interval", interval)

	// Initial update
	s.updateSet(ipset)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.updateSet(ipset)
		}
	}
}

func (s *Service) updateSet(ipset config.IPSet) {
	var v4IPs, v6IPs []string

	// Resolve all domains
	for _, domain := range ipset.Domains {
		ips, err := net.LookupIP(domain)
		if err != nil {
			s.logger.Error("Failed to resolve domain", "domain", domain, "error", err)
			continue
		}
		for _, ip := range ips {
			if ip4 := ip.To4(); ip4 != nil {
				v4IPs = append(v4IPs, ip4.String())
			} else if ip6 := ip.To16(); ip6 != nil {
				v6IPs = append(v6IPs, ip6.String())
			}
		}
	}

	// Update IPv4 set
	if len(v4IPs) > 0 {
		s.updateNftSet(ipset.Name, v4IPs)
	}

	// Update IPv6 set (convention: <name>_v6)
	if len(v6IPs) > 0 {
		s.updateNftSet(ipset.Name+"_v6", v6IPs)
	}
}

func (s *Service) updateNftSet(setName string, ips []string) {
	// Strategy: Flush and refill (Atomic updates in set are hard without diffing).

	// 1. Flush
	cmdFlush := exec.Command("nft", "flush", "set", "inet", "filter", setName)
	if err := cmdFlush.Run(); err != nil {
		s.logger.Error("Failed to flush set", "set", setName, "error", err)
		return
	}

	// 2. Add elements
	elements := strings.Join(ips, ", ")
	cmdAdd := exec.Command("nft", "add", "element", "inet", "filter", setName, fmt.Sprintf("{ %s }", elements))
	if output, err := cmdAdd.CombinedOutput(); err != nil {
		s.logger.Error("Failed to update set elements", "set", setName, "error", err, "output", string(output))
	} else {
		s.logger.Info("Updated DNS set", "set", setName, "count", len(ips))
	}
}
