// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dhcp

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/clock"
	"grimm.is/flywall/internal/errors"
	"grimm.is/flywall/internal/logging"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/services"

	"grimm.is/flywall/internal/upgrade"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"

	"grimm.is/flywall/internal/state"
)

type dhcpInstance struct {
	conn       net.PacketConn
	handler    func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4)
	auxClosers []io.Closer
}

// DNSUpdater updates DNS records.
type DNSUpdater interface {
	AddRecord(name string, ip net.IP)
	RemoveRecord(name string)
}

// LeaseListener reacts to DHCP lease events.
type LeaseListener interface {
	OnLease(mac string, ip net.IP, hostname string)
}

// ExpirationListener extends LeaseListener with expiration callbacks
type ExpirationListener interface {
	LeaseListener
	OnLeaseExpired(mac string, ip net.IP, hostname string)
}

// PacketListener for observing DHCP packets.
type PacketListener func(pkt *dhcpv4.DHCPv4, iface string, src net.Addr)

// Service manages DHCP servers for multiple scopes.
// It integrates with:
// - State Store: Persisting leases across restarts.
// - DNS Service: Updating DNS records for leased hostnames.
// - Listeners: Broadcasting lease events for UI/Logging.
// - Upgrade Manager: Handing off sockets during hot upgrades.
type Service struct {
	mu             sync.RWMutex
	servers        []*dhcpInstance
	leaseStores    []*LeaseStore // Track stores for expiration reaper
	dnsUpdater     DNSUpdater
	leaseListener  LeaseListener
	packetListener PacketListener // Passive sniffing listener
	store          state.Store
	running        bool
	stopReaper     chan struct{} // Signal to stop expiration reaper
	upgradeMgr     *upgrade.Manager
}

// SetUpgradeManager sets the upgrade manager for socket handoff.
func (s *Service) SetUpgradeManager(mgr *upgrade.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upgradeMgr = mgr
}

// SetLeaseListener sets the lease listener
func (s *Service) SetLeaseListener(l LeaseListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leaseListener = l
}

// SetPacketListener sets the packet listener for passive sniffing
func (s *Service) SetPacketListener(l PacketListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packetListener = l
}

func NewService(dnsUpdater DNSUpdater, store state.Store) *Service {
	return &Service{
		dnsUpdater: dnsUpdater,
		store:      store,
	}
}

func (s *Service) Name() string {
	return "DHCP"
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	for _, srv := range s.servers {
		go func(inst *dhcpInstance) {
			logging.WithComponent("dhcp").Debug("Starting server instance")
			s.serveDHCP(ctx, inst.conn, inst.handler)
		}(srv)
	}
	s.running = true
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		// Even if not running, close existing servers to be safe?
		// But usually servers are created on Reload.
		return nil
	}

	for _, srv := range s.servers {
		if err := srv.conn.Close(); err != nil {
			logging.WithComponent("dhcp").WithError(err).Error("Failed to stop server")
		}
		for _, c := range srv.auxClosers {
			c.Close()
		}
	}
	s.running = false
	return nil
}

// Reload reconfigures the service.
func (s *Service) Reload(cfg *config.Config) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing servers and reaper
	if s.running {
		// Stop the expiration reaper
		if s.stopReaper != nil {
			close(s.stopReaper)
			s.stopReaper = nil
		}

		for _, srv := range s.servers {
			srv.conn.Close()
			for _, c := range srv.auxClosers {
				c.Close()
			}
		}
		s.running = false
	}
	s.servers = nil     // Clear old servers
	s.leaseStores = nil // Clear old lease stores

	if cfg.DHCP == nil || !cfg.DHCP.Enabled {
		return true, nil
	}

	// Check mode
	if cfg.DHCP.Mode == "external" || cfg.DHCP.Mode == "import" {
		logging.WithComponent("dhcp").Info("Configured in external/import mode, skipping built-in server startup", "mode", cfg.DHCP.Mode)
		return true, nil
	}

	// Parse scopes (only if built-in)
	for _, scope := range cfg.DHCP.Scopes {
		var srv *dhcpInstance
		var ls *LeaseStore
		var err error

		if len(scope.RelayTo) > 0 {
			// Relay Mode
			logging.WithComponent("dhcp").Info("Configuring scope as Relay", "scope", scope.Name, "targets", scope.RelayTo)

			// For relay, we need to make sure we reuse the LeaseStore if we want to snoop?
			// createRelayHandler will snoop into *all* lease stores if we pass s.leaseStores?
			// But relay handler is a method on *Service so it has access to s.leaseStores.
			// However, we still need a LeaseStore for this subnet/scope so we can persist leases.

			// We can reuse createServer logic partially but swap the handler?
			// Let's refactor createServer to optionally return a relay handler.

			// Or just handle it here:
			ls, err = s.createLeaseStore(scope)
			if err != nil {
				return true, fmt.Errorf("failed to create lease store for scope %s: %w", scope.Name, err)
			}

			conn, err := s.bindSocket(scope)
			if err != nil {
				return true, fmt.Errorf("failed to bind socket for scope %s: %w", scope.Name, err)
			}

			// Create upstream listener for ingress traffic (replies)
			// We bind to 0.0.0.0:67 using server4 helper which enables SO_REUSEADDR,
			// allowing it to coexist with specific interface binds.
			addr := &net.UDPAddr{Port: 67}
			upstreamConn, err := server4.NewIPv4UDPConn("", addr)
			if err != nil {
				conn.Close()
				return true, fmt.Errorf("failed to bind upstream listener: %w", err)
			}

			handler, err := s.createRelayHandler(scope, scope.RelayTo, conn, upstreamConn)
			if err != nil {
				conn.Close()
				upstreamConn.Close()
				return true, fmt.Errorf("failed to create relay handler for scope %s: %w", scope.Name, err)
			}

			srv = &dhcpInstance{
				conn:       conn,
				handler:    handler,
				auxClosers: []io.Closer{upstreamConn},
			}
		} else {
			// Normal Server Mode
			srv, ls, err = s.createServer(scope, cfg.DHCP.VendorClasses)
			if err != nil {
				return true, fmt.Errorf("failed to create DHCP server for scope %s: %w", scope.Name, err)
			}
		}

		s.servers = append(s.servers, srv)
		s.leaseStores = append(s.leaseStores, ls)
	}

	// Restart servers
	for _, srv := range s.servers {
		go func(inst *dhcpInstance) {
			logging.WithComponent("dhcp").Debug("Starting server instance")
			s.serveDHCP(context.Background(), inst.conn, inst.handler) // Background ctx for Handover
		}(srv)
	}

	// Start expiration reaper
	// Calculate reaper interval based on minimum lease time
	minInterval := 1 * time.Minute
	for _, ls := range s.leaseStores {
		lt := ls.getLeaseTime()
		if lt < minInterval {
			minInterval = lt
		}
	}
	// Run reaper at least at 1/2 of min lease time, or max 1 minute, min 1 second
	reaperInterval := minInterval / 2
	if reaperInterval > 1*time.Minute {
		reaperInterval = 1 * time.Minute
	}
	if reaperInterval < 1*time.Second {
		reaperInterval = 1 * time.Second
	}

	logging.WithComponent("dhcp").Debug("Starting expiration reaper", "interval", reaperInterval)

	s.stopReaper = make(chan struct{})
	go s.runExpirationReaper(s.stopReaper, reaperInterval)

	s.running = true

	return true, nil
}

// runExpirationReaper periodically checks for and removes expired leases
func (s *Service) runExpirationReaper(stop <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger := logging.WithComponent("dhcp")
	logger.Debug("Expiration reaper started")

	for {
		select {
		case <-ticker.C:
			s.expireLeases()
		case <-stop:
			logger.Debug("Expiration reaper stopped")
			return
		}
	}
}

// expireLeases checks all lease stores for expired leases
func (s *Service) expireLeases() {
	s.mu.RLock()
	stores := s.leaseStores
	dnsUpdater := s.dnsUpdater
	listener := s.leaseListener
	s.mu.RUnlock()

	var expListener ExpirationListener
	if listener != nil {
		if el, ok := listener.(ExpirationListener); ok {
			expListener = el
		}
	}

	totalExpired := 0
	for _, store := range stores {
		totalExpired += store.ExpireLeases(dnsUpdater, expListener)
	}

	if totalExpired > 0 {
		logging.WithComponent("dhcp").Info("Expired leases", "count", totalExpired)
	}
}

func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Service) Status() services.ServiceStatus {
	return services.ServiceStatus{
		Name:    s.Name(),
		Running: s.IsRunning(),
	}
}

type LeaseExpiry struct {
	IP       net.IP
	Hostname string
	Expires  time.Time
}

// Lease storage (simple in-memory)
// Handles allocation strategy:
// 1. Static Reservations
// 2. Existing Lease reuse
// 3. First-available IP from pool
type LeaseStore struct {
	sync.Mutex
	Leases       map[string]net.IP                 // MAC -> IP
	TakenIPs     map[string]string                 // IP (string) -> MAC (for O(1) isTaken lookup)
	Reservations map[string]config.DHCPReservation // MAC -> Reservation
	ReservedIPs  map[string]string                 // IP (string) -> MAC
	RangeStart   net.IP
	RangeEnd     net.IP
	Subnet       *net.IPNet        // Interface subnet for validation
	bucket       *state.DHCPBucket // Persistent storage

	// Expiration support
	clock       clock.Clock          // Injectable clock for testing
	leaseTime   time.Duration        // Default lease duration
	hostnames   map[string]string    // MAC -> hostname for DNS cleanup
	leaseExpiry map[string]time.Time // MAC -> expiration time
}

func (s *LeaseStore) Allocate(mac string) (net.IP, error) {
	s.Lock()
	defer s.Unlock()

	// 1. Check for static reservation
	if res, ok := s.Reservations[mac]; ok {
		// Parse IP from reservation
		ip := net.ParseIP(res.IP).To4()
		if ip != nil {
			// Validate reservation is in subnet
			if s.Subnet != nil && !s.Subnet.Contains(ip) {
				return nil, fmt.Errorf("reserved IP %s is not in subnet %s", ip, s.Subnet)
			}
			return ip, nil
		}
	}

	// 2. Check existing dynamic lease
	if ip, ok := s.Leases[mac]; ok {
		// Re-validate against current subnet in case of config change
		if s.Subnet == nil || s.Subnet.Contains(ip) {
			return ip, nil
		}
		// If no longer in subnet, proceed to re-allocate
		logging.WithComponent("dhcp").Warn("Existing lease no longer in subnet", "ip", ip, "mac", mac, "subnet", s.Subnet)
		delete(s.Leases, mac)
		delete(s.TakenIPs, ip.String())
	}

	// 3. Allocate new dynamic IP (Naive linear scan)
	for ip := s.RangeStart; !ipMatches(ip, s.RangeEnd); ip = incIP(ip) {
		ipStr := ip.String()

		// Validate against subnet if configured
		if s.Subnet != nil && !s.Subnet.Contains(ip) {
			continue
		}

		// Skip if this IP is reserved for another MAC
		if _, reserved := s.ReservedIPs[ipStr]; reserved {
			continue
		}

		// Skip if currently leased
		if !s.isTaken(ip) {
			newIP := make(net.IP, len(ip))
			copy(newIP, ip)

			// Persist first
			if err := s.persistLease(mac, newIP, "hostname-unknown"); err != nil {
				logging.WithComponent("dhcp").WithError(err).Error("Failed to persist lease")
				// Continue anyway or fail? Fail to ensure safety.
				return nil, errors.Wrap(err, errors.KindInternal, "failed to persist lease")
			}

			s.Leases[mac] = newIP
			s.TakenIPs[newIP.String()] = mac // Maintain reverse lookup
			s.setLeaseExpiry(mac)
			return newIP, nil
		}
	}

	// Check the last one (RangeEnd)
	if s.Subnet == nil || s.Subnet.Contains(s.RangeEnd) {
		if _, reserved := s.ReservedIPs[s.RangeEnd.String()]; !reserved && !s.isTaken(s.RangeEnd) {
			newIP := make(net.IP, len(s.RangeEnd))
			copy(newIP, s.RangeEnd)

			// Persist
			if err := s.persistLease(mac, newIP, "hostname-unknown"); err != nil {
				return nil, errors.Wrap(err, errors.KindInternal, "failed to persist lease")
			}

			s.Leases[mac] = newIP
			s.TakenIPs[newIP.String()] = mac // Maintain reverse lookup
			s.setLeaseExpiry(mac)
			return newIP, nil
		}
	}

	return nil, fmt.Errorf("no IPs available")
}

func (s *LeaseStore) isTaken(ip net.IP) bool {
	// O(1) lookup using TakenIPs reverse map
	_, exists := s.TakenIPs[ip.String()]
	return exists
}

func incIP(ip net.IP) net.IP {
	ret := make(net.IP, len(ip))
	copy(ret, ip)
	for i := len(ret) - 1; i >= 0; i-- {
		ret[i]++
		if ret[i] > 0 {
			break
		}
	}
	return ret
}

func ipMatches(a, b net.IP) bool {
	return a.Equal(b)
}

func (s *LeaseStore) persistLease(mac string, ip net.IP, hostname string) error {
	if s.bucket == nil {
		return nil
	}

	lease := &state.DHCPLease{
		MAC:        mac,
		IP:         ip.String(),
		Hostname:   hostname,
		LeaseStart: clock.Now(),
		LeaseEnd:   clock.Now().Add(s.getLeaseTime()),
	}

	// Synchronous write to ensure persistence
	if err := s.bucket.Set(lease); err != nil {
		return errors.Wrap(err, errors.KindInternal, "failed to persist lease to state store")
	}
	logging.WithComponent("dhcp").Debug("Persisted lease", "mac", mac, "ip", ip)
	return nil
}

// getNow returns the current time using injected clock or real time
func (s *LeaseStore) getNow() time.Time {
	if s.clock != nil {
		return s.clock.Now()
	}
	return clock.Now()
}

// getLeaseTime returns the configured lease time or default 24 hours
func (s *LeaseStore) getLeaseTime() time.Duration {
	if s.leaseTime > 0 {
		return s.leaseTime
	}
	logging.WithComponent("dhcp").Debug("getLeaseTime returning default 24h", "lease_time", s.leaseTime)
	return 24 * time.Hour
}

// SetHostname associates a hostname with a MAC for expiration callbacks
func (s *LeaseStore) SetHostname(mac string, hostname string) {
	s.Lock()
	defer s.Unlock()
	if s.hostnames == nil {
		s.hostnames = make(map[string]string)
	}
	s.hostnames[mac] = hostname
}

// setLeaseExpiry records when a lease expires (called during allocation)
func (s *LeaseStore) setLeaseExpiry(mac string) {
	// Called while already holding lock
	if s.leaseExpiry == nil {
		s.leaseExpiry = make(map[string]time.Time)
	}
	s.leaseExpiry[mac] = s.getNow().Add(s.getLeaseTime())
}

// RenewLease extends the lease for an existing MAC address
func (s *LeaseStore) RenewLease(mac string) error {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.Leases[mac]; !ok {
		return fmt.Errorf("no active lease for MAC %s", mac)
	}

	// Extend expiration from now
	if s.leaseExpiry == nil {
		s.leaseExpiry = make(map[string]time.Time)
	}
	s.leaseExpiry[mac] = s.getNow().Add(s.getLeaseTime())

	logging.WithComponent("dhcp").Info("Renewed lease", "mac", mac, "expiry", s.leaseExpiry[mac])
	return nil
}

// ExpireLeases checks for expired leases and removes them
// Returns the number of leases expired
func (s *LeaseStore) ExpireLeases(dnsUpdater DNSUpdater, listener ExpirationListener) int {
	s.Lock()
	defer s.Unlock()

	if s.leaseExpiry == nil {
		return 0
	}

	now := s.getNow()
	expired := 0

	for mac, expiry := range s.leaseExpiry {
		if now.After(expiry) {
			ip := s.Leases[mac]
			hostname := ""
			if s.hostnames != nil {
				hostname = s.hostnames[mac]
			}

			// Delete from persistent store first. If this fails (e.g. SQLITE_BUSY
			// under I/O pressure), skip the in-memory cleanup and let the reaper
			// retry on the next tick. This prevents state divergence where the
			// allocator thinks an IP is free but the DB still has the lease.
			if s.bucket != nil {
				if err := s.bucket.Delete(mac); err != nil {
					logging.WithComponent("dhcp").WithError(err).Warn("Failed to delete expired lease from store, will retry", "mac", mac)
					continue
				}
			}

			// DB delete succeeded (or no bucket) — now clean up in-memory state
			delete(s.Leases, mac)
			if ip != nil {
				delete(s.TakenIPs, ip.String()) // Maintain reverse lookup
			}
			delete(s.leaseExpiry, mac)
			if s.hostnames != nil {
				delete(s.hostnames, mac)
			}

			// Remove DNS record if updater provided
			if dnsUpdater != nil && hostname != "" {
				dnsUpdater.RemoveRecord(hostname)
			}

			// Notify listener
			if listener != nil {
				listener.OnLeaseExpired(mac, ip, hostname)
			}

			logging.WithComponent("dhcp").Info("Expired lease", "mac", mac, "ip", ip)
			expired++
		}
	}

	return expired
}

func (s *Service) createServer(scope config.DHCPScope, vendorClasses []config.DHCPVendorClass) (*dhcpInstance, *LeaseStore, error) {
	ls, err := s.createLeaseStore(scope)
	if err != nil {
		return nil, nil, err
	}

	// We need routerIP for handler
	routerIP := net.ParseIP(scope.Router).To4()

	handler := func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
		// Notify passive packet listener
		s.mu.RLock()
		pl := s.packetListener
		s.mu.RUnlock()
		if pl != nil {
			// Run in goroutine to not block DHCP server logic
			go pl(m, scope.Interface, peer)
		}

		// Helper to determine destination address
		// If peer is 0.0.0.0 (DHCP Discover), we MUST reply to Broadcast.
		dest := peer
		if udpAddr, ok := peer.(*net.UDPAddr); ok {
			if udpAddr.IP.IsUnspecified() || udpAddr.IP.Equal(net.IPv4zero) {
				dest = &net.UDPAddr{
					IP:   net.IPv4bcast,
					Port: 68,
				}
				// logging.Debug("[DHCP] Peer is 0.0.0.0, forcing broadcast reply to 255.255.255.255:68")
			}
		}

		switch m.MessageType() {
		case dhcpv4.MessageTypeDiscover:
			offer, err := handleDiscover(m, ls, scope, routerIP, vendorClasses)
			if err != nil {
				logging.WithComponent("dhcp").WithError(err).Error("Discover error")
				return
			}
			if _, err := conn.WriteTo(offer.ToBytes(), dest); err != nil {
				logging.WithComponent("dhcp").WithError(err).Error("WriteOffer error", "dest", dest)
			}
		case dhcpv4.MessageTypeRequest:
			ack, err := handleRequest(m, ls, scope, routerIP, s.dnsUpdater, s.leaseListener, vendorClasses)
			if err != nil {
				logging.WithComponent("dhcp").WithError(err).Error("Request error")
				return
			}
			if _, err := conn.WriteTo(ack.ToBytes(), dest); err != nil {
				logging.WithComponent("dhcp").WithError(err).Error("WriteAck error", "dest", dest)
			}
		}
	}

	// Bind socket
	conn, err := s.bindSocket(scope)
	if err != nil {
		return nil, nil, err
	}

	// Manually construct server instance
	srv := &dhcpInstance{
		conn:    conn,
		handler: handler,
	}

	return srv, ls, nil
}
func (s *Service) createLeaseStore(scope config.DHCPScope) (*LeaseStore, error) {
	startIP := net.ParseIP(scope.RangeStart).To4()
	endIP := net.ParseIP(scope.RangeEnd).To4()
	routerIP := net.ParseIP(scope.Router).To4()

	if startIP == nil || endIP == nil || routerIP == nil {
		return nil, errors.New(errors.KindValidation, "invalid IP configuration for scope")
	}

	// Setup Lease Store with Reservations
	ls := &LeaseStore{
		Leases:       make(map[string]net.IP),
		TakenIPs:     make(map[string]string), // O(1) reverse lookup
		Reservations: make(map[string]config.DHCPReservation),
		ReservedIPs:  make(map[string]string),
		RangeStart:   startIP,
		RangeEnd:     endIP,
	}

	logger := logging.WithComponent("dhcp")

	// Determine subnet from router IP and interface mask
	mask, err := getInterfaceMask(scope.Interface, routerIP)
	if err != nil {
		// Fallback to /24 if interface lookup fails (safe default, logs warning)
		logger.WithError(err).Warn("Failed to get interface mask, falling back to /24", "interface", scope.Interface)
		mask = net.IPv4Mask(255, 255, 255, 0)
	}
	ls.Subnet = &net.IPNet{
		IP:   routerIP.Mask(mask),
		Mask: mask,
	}

	// Parse lease time
	logger.Debug("Scope Config LeaseTime", "scope", scope.Name, "lease_time", scope.LeaseTime)
	if scope.LeaseTime != "" {
		d, err := time.ParseDuration(scope.LeaseTime)
		if err != nil {
			logger.WithError(err).Warn("Invalid lease_time for scope, using default 24h", "scope", scope.Name, "lease_time", scope.LeaseTime)
		} else {
			ls.leaseTime = d
		}
	}

	// Initialize bucket and load existing leases
	if s.store != nil {
		bucket, err := state.NewDHCPBucket(s.store)
		if err != nil {
			logger.WithError(err).Warn("Failed to create/open DHCP bucket")
		} else {
			ls.bucket = bucket
			leases, err := bucket.List()
			if err != nil {
				logger.WithError(err).Warn("Failed to list existing leases")
			} else {
				// Load leases into memory
				for _, l := range leases {
					ip := net.ParseIP(l.IP).To4()
					if ip != nil {
						// Note: We intentionally don't check if IP is within this scope's range.
						// Leases may span pool boundaries if config changed, and we preserve them.
						ls.Leases[l.MAC] = ip
						ls.TakenIPs[ip.String()] = l.MAC // Populate reverse lookup
					}
				}
				logger.Info("Loaded leases from state store", "count", len(leases))
			}
		}
	}

	// Populate reservations
	for _, res := range scope.Reservations {
		ip := net.ParseIP(res.IP).To4()
		if ip != nil {
			ls.Reservations[res.MAC] = res
			ls.ReservedIPs[ip.String()] = res.MAC
		}
	}

	return ls, nil
}

func (s *Service) bindSocket(scope config.DHCPScope) (net.PacketConn, error) {
	logger := logging.WithComponent("dhcp")
	linkName := "dhcp-v4" // Single listener for now (0.0.0.0)

	var conn net.PacketConn
	if s.upgradeMgr != nil {
		if existing, ok := s.upgradeMgr.GetPacketConn(linkName); ok {
			conn = existing
			logger.Info("Inherited socket", "link", linkName)
		}
	}

	if conn == nil {
		addr := &net.UDPAddr{
			IP:   net.IPv4zero,
			Port: 67,
		}

		udpConn, err := server4.NewIPv4UDPConn(scope.Interface, addr)
		if err != nil {
			return nil, err
		}
		conn = udpConn

		if s.upgradeMgr != nil {
			s.upgradeMgr.RegisterPacketConn(linkName, conn)
		}
	}
	return conn, nil
}

// serveDHCP runs the read loop for a DHCP server instance
func (s *Service) serveDHCP(ctx context.Context, conn net.PacketConn, handler func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4)) {
	buf := make([]byte, 4096) // Standard UDP buffer
	logger := logging.WithComponent("dhcp")

	for {
		select {
		case <-ctx.Done():
			logger.Debug("Server loop stopping due to context cancellation")
			return
		default:
			// Set a short deadline to check context periodically
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
					continue
				}
				if s.running {
					logger.WithError(err).Error("Read error")
				}
				return
			}

			pkt, err := dhcpv4.FromBytes(buf[:n])
			if err != nil {
				continue
			}

			handler(conn, addr, pkt)
		}
	}
}

func handleDiscover(m *dhcpv4.DHCPv4, store *LeaseStore, scope config.DHCPScope, routerIP net.IP, vendorClasses []config.DHCPVendorClass) (*dhcpv4.DHCPv4, error) {
	// Allocate IP
	mac := m.ClientHWAddr.String()
	ip, err := store.Allocate(mac)
	if err != nil {
		return nil, errors.Wrap(err, errors.KindInternal, "failed to allocate IP during discover")
	}

	// Prepare options
	opts := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithYourIP(ip),
		dhcpv4.WithServerIP(routerIP),
		dhcpv4.WithRouter(routerIP),
		dhcpv4.WithNetmask(net.IPv4Mask(255, 255, 255, 0)), // Assuming /24 for now
		dhcpv4.WithDNS(parseIPs(scope.DNS)...),
		dhcpv4.WithLeaseTime(uint32(store.getLeaseTime().Seconds())),
	}

	if scope.Domain != "" {
		opts = append(opts, dhcpv4.WithDomainSearchList(scope.Domain))
	}

	// Add scope custom options
	for k, v := range scope.Options {
		opt, err := parseOption(k, v)
		if err != nil {
			logging.WithComponent("dhcp").WithError(err).Warn("Failed to parse scope option", "option", k, "value", v)
			continue
		}
		opts = append(opts, dhcpv4.WithOption(opt))
	}

	// Add per-host custom options
	store.Lock()
	res, hasRes := store.Reservations[mac]
	store.Unlock()

	if hasRes {
		for k, v := range res.Options {
			opt, err := parseOption(k, v)
			if err != nil {
				logging.WithComponent("dhcp").WithError(err).Warn("Failed to parse host option", "mac", mac, "option", k, "value", v)
			}
			opts = append(opts, dhcpv4.WithOption(opt))
		}
	}

	// Add vendor class options (Option 60)
	applyVendorOptions(m, vendorClasses, &opts)

	return dhcpv4.NewReplyFromRequest(m, opts...)
}

// handleRequest processes a DHCP Request packet.
// It finalizes the lease allocation, updates DNS records (if enabled),
// and sends a DHCP ACK or NAK.
func handleRequest(m *dhcpv4.DHCPv4, store *LeaseStore, scope config.DHCPScope, routerIP net.IP, dnsUpdater DNSUpdater, listener LeaseListener, vendorClasses []config.DHCPVendorClass) (*dhcpv4.DHCPv4, error) {
	mac := m.ClientHWAddr.String()

	// Verify the requested IP matches what we would allocate (or have allocated)
	requestedIP := m.RequestedIPAddress()
	if requestedIP == nil {
		requestedIP = m.ClientIPAddr
	}

	allocatedIP, err := store.Allocate(mac)
	if err != nil {
		return nil, errors.Wrap(err, errors.KindInternal, "failed to allocate IP during request")
	}

	if !allocatedIP.Equal(requestedIP) && !requestedIP.IsUnspecified() {
		// Send DHCP NAK to tell client their IP is invalid
		// This causes immediate DISCOVER rather than waiting for timeout
		logging.WithComponent("dhcp").Warn("Sending NAK: client requested invalid IP", "mac", mac, "requested", requestedIP, "expected", allocatedIP)
		nakOpts := []dhcpv4.Modifier{
			dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
			dhcpv4.WithServerIP(routerIP),
		}
		return dhcpv4.NewReplyFromRequest(m, nakOpts...)
	}

	// Retrieve reservation for hostname/options
	store.Lock()
	res, hasRes := store.Reservations[mac]
	store.Unlock()

	// Handle DNS Integration
	hostname := m.HostName()
	if hostname == "" && hasRes && res.Hostname != "" {
		hostname = res.Hostname
	}

	if hostname != "" && dnsUpdater != nil {
		// If domain is set, append it
		if scope.Domain != "" {
			hostname = hostname + "." + scope.Domain
		}
		dnsUpdater.AddRecord(hostname, allocatedIP)
	}

	// Prepare options
	opts := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(allocatedIP),
		dhcpv4.WithServerIP(routerIP),
		dhcpv4.WithRouter(routerIP),
		dhcpv4.WithNetmask(net.IPv4Mask(255, 255, 255, 0)),
		dhcpv4.WithDNS(parseIPs(scope.DNS)...),
		dhcpv4.WithLeaseTime(uint32(store.getLeaseTime().Seconds())),
	}

	if scope.Domain != "" {
		opts = append(opts, dhcpv4.WithDomainSearchList(scope.Domain))
	}

	// Add scope custom options
	for k, v := range scope.Options {
		opt, err := parseOption(k, v)
		if err != nil {
			logging.WithComponent("dhcp").WithError(err).Warn("Failed to parse scope option", "option", k, "value", v)
			continue
		}
		opts = append(opts, dhcpv4.WithOption(opt))
	}

	// Add per-host custom options
	if hasRes {
		for k, v := range res.Options {
			opt, err := parseOption(k, v)
			if err != nil {
				logging.WithComponent("dhcp").WithError(err).Warn("Failed to parse host option", "mac", mac, "option", k, "value", v)
				continue
			}
			opts = append(opts, dhcpv4.WithOption(opt))
		}
	}

	// Add vendor class options (Option 60)
	applyVendorOptions(m, vendorClasses, &opts)

	// Trigger listener
	if listener != nil {
		go listener.OnLease(mac, allocatedIP, hostname)
	}

	return dhcpv4.NewReplyFromRequest(m, opts...)
}

// Lease represents a DHCP lease for external consumption
type Lease struct {
	MAC        string
	IP         net.IP
	Hostname   string
	Expiration time.Time
}

// GetLeases returns all active leases across all scopes
func (s *Service) GetLeases() []Lease {
	s.mu.RLock()
	stores := s.leaseStores
	s.mu.RUnlock()

	var leases []Lease

	for _, store := range stores {
		store.Lock()
		for mac, ip := range store.Leases {
			l := Lease{
				MAC: mac,
				IP:  ip,
			}
			if store.leaseExpiry != nil {
				if expiry, ok := store.leaseExpiry[mac]; ok {
					l.Expiration = expiry
				}
			}
			if store.hostnames != nil {
				if hostname, ok := store.hostnames[mac]; ok {
					l.Hostname = hostname
				}
			}
			leases = append(leases, l)
		}
		store.Unlock()
	}
	return leases
}

// applyVendorOptions checks if the message has a Vendor Class Identifier (Option 60)
// and applies matching vendor class options to the response modifiers.
func applyVendorOptions(m *dhcpv4.DHCPv4, vendorClasses []config.DHCPVendorClass, opts *[]dhcpv4.Modifier) {
	vendorClassID := m.GetOneOption(dhcpv4.OptionClassIdentifier)
	if vendorClassID == nil {
		return
	}

	vcStr := string(vendorClassID)
	for _, vc := range vendorClasses {
		// Use partial match (contains) as many vendors append version info
		if strings.Contains(vcStr, vc.Identifier) {
			for k, v := range vc.Options {
				opt, err := parseOption(k, v)
				if err != nil {
					logging.WithComponent("dhcp").WithError(err).Warn("Failed to parse vendor class option", "vendor", vc.Name, "option", k, "value", v)
					continue
				}
				*opts = append(*opts, dhcpv4.WithOption(opt))
			}
			// Only apply first matching vendor class
			break
		}
	}
}

// getInterfaceMask retrieves the subnet mask for the given interface and IP.
func getInterfaceMask(ifaceName string, ip net.IP) (net.IPMask, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("interface not found: %w", err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses: %w", err)
	}

	for _, addr := range addrs {
		// Check if this is an IPNet (should be)
		if ipNet, ok := addr.(*net.IPNet); ok {
			// Check if IP matches (ignoring mask)
			if ipNet.IP.Equal(ip) {
				return ipNet.Mask, nil
			}
		}
	}

	return nil, fmt.Errorf("IP %s not found on interface %s", ip, ifaceName)
}
