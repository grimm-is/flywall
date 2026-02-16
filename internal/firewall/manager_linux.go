// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux
// +build linux

package firewall

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/install"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/errors"
	"grimm.is/flywall/internal/logging"

	"path/filepath"

	"github.com/google/nftables"
)

// Manager handles firewall rules.
type Manager struct {
	conn NFTablesConn

	// State for integrity monitoring
	mu             sync.RWMutex
	baseConfig     *Config // Config from file/API without dynamic rules
	currentConfig  *Config // Currently applied config (including dynamic)
	dynamicRules   []config.NATRule
	scheduledRules map[string]config.ScheduledRule // Active scheduled rules
	expectedGenID  uint32
	monitorEnabled bool

	// Safe Mode: pre-rendered ruleset for instant emergency lockdown
	safeModeScript string
	inSafeMode     bool

	logger   *logging.Logger
	cacheDir string

	// Integrity restore callback
	restoreCallback func()
}

// NewManager creates a new firewall manager with default dependencies.
func NewManager(logger *logging.Logger, cacheDir string) (*Manager, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, err
	}
	return NewManagerWithConn(NewRealNFTablesConn(conn), logger, cacheDir), nil
}

// NewManagerWithConn creates a new firewall manager with injected dependencies.
func NewManagerWithConn(conn NFTablesConn, logger *logging.Logger, cacheDir string) *Manager {
	if logger == nil {
		logger = logging.New(logging.DefaultConfig())
	}
	if cacheDir == "" {
		cacheDir = filepath.Join(install.GetStateDir(), "iplists")
	}
	return &Manager{
		conn:     conn,
		logger:   logger,
		cacheDir: cacheDir,
	}
}

// ApplyConfig applies the firewall configuration atomically.
// The entire ruleset is built as a script and applied in a single atomic operation,
// ensuring no window of vulnerability during rule updates.
func (m *Manager) ApplyConfig(cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reconstruct strict config for internal builders
	// Use config directly
	globalCfg := cfg

	// Update current config reference
	// Convert to local structure
	localCfg := cfg
	m.baseConfig = localCfg // Store base config for re-application

	// Create effective config by merging dynamic rules
	effectiveCfg := *localCfg
	// Deep copy NAT rules to avoid modifying base
	effectiveCfg.NAT = make([]config.NATRule, len(localCfg.NAT)+len(m.dynamicRules))
	copy(effectiveCfg.NAT, localCfg.NAT)
	copy(effectiveCfg.NAT[len(localCfg.NAT):], m.dynamicRules)

	// DEBUG: Trace protections
	if len(effectiveCfg.Protections) > 0 {
		m.logger.Info("ApplyConfig: Found protection rules", "count", len(effectiveCfg.Protections))
		for _, p := range effectiveCfg.Protections {
			m.logger.Info(" - Protection", "interface", p.Interface, "enabled", p.Enabled)
		}
	} else {
		m.logger.Warn("ApplyConfig: No protection rules found in config")
	}

	// Merge scheduled rules into policies
	// We need to deep copy policies too if we modify them
	if len(m.scheduledRules) > 0 {
		newPolicies := make([]config.Policy, len(localCfg.Policies))
		copy(newPolicies, localCfg.Policies)
		effectiveCfg.Policies = newPolicies

		for i := range effectiveCfg.Policies {
			pol := &effectiveCfg.Policies[i]
			// Copy rules slice to avoid mutating base config rules underlying array
			newRules := make([]config.PolicyRule, len(pol.Rules))
			copy(newRules, pol.Rules)
			pol.Rules = newRules

			for ruleName, schedRule := range m.scheduledRules {
				if schedRule.PolicyName == pol.Name {
					// Log injection at debug level to avoid spam
					m.logger.Debug("Injecting scheduled rule", "name", ruleName, "policy", pol.Name)

					// Inject rule at end of policy.
					// Design choice: Scheduled rules are appended after static rules.
					// For priority ordering, use the policy's rule order instead.
					schedRule.Rule.Comment = fmt.Sprintf("[Schedule: %s] %s", ruleName, schedRule.Rule.Comment)
					pol.Rules = append(pol.Rules, schedRule.Rule)
				}
			}
		}
	}

	m.currentConfig = &effectiveCfg

	if globalCfg.Features != nil {
		m.monitorEnabled = globalCfg.Features.IntegrityMonitoring
	} else {
		m.monitorEnabled = false
	}

	// Build the complete ruleset as an atomic script
	// 0. Pre-validate configuration to prevent injection
	if err := m.validateConfig(localCfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// 1. Resolve IPSets (Download/Fetch)
	// This happens BEFORE rule generation to ensure the set
	// elements are included in the atomic script.
	resolvedIPSets, err := m.resolveIPSets(localCfg)
	if err != nil {
		return fmt.Errorf("ipset resolution failed: %w", err)
	}

	finalScript, err := m.GenerateRules(&effectiveCfg, resolvedIPSets)
	if err != nil {
		return err
	}

	// 4. Validate script before applying
	applier := NewAtomicApplier()
	if err := applier.ValidateScript(finalScript); err != nil {
		// Dump script to log for debugging if validation fails
		// Truncate to avoid massive logs (first 1000 chars)
		limit := 1000
		if len(finalScript) < limit {
			limit = len(finalScript)
		}
		m.logger.WithError(err).WithFields(map[string]any{
			"script_snippet": finalScript[:limit],
		}).Error("Ruleset validation failed")
		return errors.Wrap(err, errors.KindValidation, "ruleset validation failed")
	}

	// 5. Apply atomically
	if err := applier.ApplyScript(finalScript); err != nil {
		return fmt.Errorf("atomic apply failed: %w", err)
	}

	// 6. IPSet logic removed (handled atomically above)

	// Update expectedGenID for integrity monitor
	if m.monitorEnabled {
		genID, err := m.getRulesetGenID(m.conn)
		if err == nil {
			m.expectedGenID = genID
		} else {
			m.logger.Warn("Failed to update expectedGenID", "error", err)
		}
	}

	// 7. Enable route_localnet to allow routing to 169.254.x.x (sandbox)
	// This is required because the kernel treats Link-Local as non-routable by default.
	if err := m.enableRouteLocalnet(localCfg); err != nil {
		m.logger.Warn("Failed to enable route_localnet", "error", err)
	}

	return nil
}

// resolveIPSets fetches all IPSet data (static strings, downloads)
// and returns a map of set contents.
func (m *Manager) resolveIPSets(cfg *Config) (map[string][]string, error) {
	resolved := make(map[string][]string)

	// Initialize ListManager
	// We pass empty string for config file here as we rely on defaults/config dir behavior
	// In a real scenario, we might want to pass a configured path from cfg.Global.ListConfig
	listMgr, err := NewListManager(m.cacheDir, m.logger, "")
	if err != nil {
		m.logger.Warn("Failed to initialize ListManager (using defaults)", "error", err)
	}

	// Safety limit to prevent memory exhaustion DoS
	const MaxTotalIPs = 500000
	totalIPs := 0

	for _, ipset := range cfg.IPSets {
		var entries []string

		// 1. Static entries
		if len(ipset.Entries) > 0 {
			entries = append(entries, ipset.Entries...)
		}

		// 2. Managed List (Generic or Legacy FireHOL)
		listName := ipset.ManagedList
		if listName == "" {
			listName = ipset.FireHOLList // Fallback for backward compatibility
		}

		if listName != "" && listMgr != nil {
			ips, err := listMgr.DownloadList(listName)
			if err != nil {
				m.logger.Warn("failed to download managed list (using static entries only)", "list", listName, "error", err)
			} else {
				entries = append(entries, ips...)
			}
		}

		// 3. Custom URL
		if ipset.URL != "" && listMgr != nil {
			ips, err := listMgr.DownloadFromURL(ipset.URL)
			if err != nil {
				m.logger.Warn("failed to download from URL (using static entries only)", "url", ipset.URL, "error", err)
			} else {
				entries = append(entries, ips...)
			}
		}

		totalIPs += len(entries)
		if totalIPs > MaxTotalIPs {
			return nil, fmt.Errorf("ipset total entry count exceeds limit (%d)", MaxTotalIPs)
		}

		if len(entries) > 0 {
			resolved[ipset.Name] = entries
		}
	}
	return resolved, nil
}

// enableRouteLocalnet enables route_localnet on interfaces where Web/API access is required.
func (m *Manager) enableRouteLocalnet(cfg *Config) error {
	// Helper to write file
	writeSysctl := func(path, value string) error {
		// Read first to avoid unnecessary writes (reduction of side effects)
		current, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(current)) == value {
			return nil
		}
		return os.WriteFile(path, []byte(value), 0644)
	}

	// We no longer enable globally on all/default to adhere to least privilege.
	// Only interfaces that actually need to route to the sandbox (Link-Local) will have it enabled.

	// Helper to check if an interface needs route_localnet
	needsRouteLocalnet := func(iface config.Interface) bool {
		// 1. Legacy Interface Config
		if iface.AccessWebUI {
			return true
		}
		if iface.Management != nil && (iface.Management.Web || iface.Management.WebUI || iface.Management.API) {
			return true
		}

		// 2. Zone Config
		// Find zone for this interface
		if iface.Zone != "" {
			for _, z := range cfg.Zones {
				if strings.EqualFold(z.Name, iface.Zone) {
					if z.Management != nil && (z.Management.Web || z.Management.WebUI || z.Management.API) {
						return true
					}
					break
				}
			}
		}
		return false
	}

	// Enable on specific interfaces
	for _, iface := range cfg.Interfaces {
		// sanitization for path safety not strictly needed as interface names are validated differently,
		// but good practice. Assuming verified interface names.
		path := fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/route_localnet", iface.Name)

		// Only enable if needed
		targetValue := "0"
		if needsRouteLocalnet(iface) {
			targetValue = "1"
		}

		if _, err := os.Stat(path); err == nil {
			if err := writeSysctl(path, targetValue); err != nil {
				m.logger.Warn("Failed to set route_localnet on interface", "interface", iface.Name, "value", targetValue, "error", err)
			} else {
				if targetValue == "1" {
					m.logger.Debug("Enabled route_localnet on interface", "interface", iface.Name)
				}
			}
		}
	}
	return nil
}

// AddDynamicNATRule adds a dynamic NAT rule (e.g., for UPnP) and reapplies the firewall.
func (m *Manager) AddDynamicNATRule(rule config.NATRule) error {
	m.mu.Lock()
	m.dynamicRules = append(m.dynamicRules, rule)
	// Create a local copy of base parameters to release lock during ApplyConfig
	if m.baseConfig == nil {
		m.mu.Unlock()
		return fmt.Errorf("cannot add dynamic rule: firewall not initialized")
	}
	base := m.baseConfig
	m.mu.Unlock()

	// Re-apply using base config (ApplyConfig will merge the new dynamic rule)
	return m.ApplyConfig(base)
}

// RemoveDynamicNATRule removes dynamic NAT rules matching the predicate and reapplies.
func (m *Manager) RemoveDynamicNATRule(match func(config.NATRule) bool) error {
	m.mu.Lock()
	var newRules []config.NATRule
	changed := false
	for _, r := range m.dynamicRules {
		if match(r) {
			changed = true
			continue
		}
		newRules = append(newRules, r)
	}
	if !changed {
		m.mu.Unlock()
		return nil
	}
	m.dynamicRules = newRules

	if m.baseConfig == nil {
		m.mu.Unlock()
		return fmt.Errorf("cannot remove dynamic rule: firewall not initialized")
	}
	base := m.baseConfig
	m.mu.Unlock()

	return m.ApplyConfig(base)
}

// ApplyScheduledRule adds or removes a scheduled rule and reapplies the firewall.
func (m *Manager) ApplyScheduledRule(rule config.ScheduledRule, enabled bool) error {
	m.mu.Lock()
	if m.scheduledRules == nil {
		m.scheduledRules = make(map[string]config.ScheduledRule)
	}

	if enabled {
		m.scheduledRules[rule.Name] = rule
		m.logger.Info("Scheduled rule enabled", "name", rule.Name)
	} else {
		delete(m.scheduledRules, rule.Name)
		m.logger.Info("Scheduled rule disabled", "name", rule.Name)
	}

	// Create a local copy of base parameters to release lock during ApplyConfig
	if m.baseConfig == nil {
		m.mu.Unlock()
		return fmt.Errorf("cannot apply scheduled rule: firewall not initialized")
	}
	base := m.baseConfig
	m.mu.Unlock()

	// Re-apply using base config (ApplyConfig will merge the scheduled rules)
	return m.ApplyConfig(base)
}

// validateConfig checks against injection attacks by enforcing strict naming
func (m *Manager) validateConfig(cfg *Config) error {
	// Validate Zone Names
	for _, zone := range cfg.Zones {
		if !isValidIdentifier(zone.Name) {
			return errors.Errorf(errors.KindValidation, "invalid zone name '%s': must match [a-zA-Z0-9_.-]+", zone.Name)
		}
		// Validate Interfaces in Zone
		for _, m := range zone.Matches {
			if m.Interface == "" {
				continue // Skip empty matches or non-interface matches
			}
			if !isValidIdentifier(m.Interface) {
				return errors.Errorf(errors.KindValidation, "invalid interface name '%s' in zone '%s'", m.Interface, zone.Name)
			}
		}
	}

	// Validate Interface Objects
	for _, iface := range cfg.Interfaces {
		if !isValidIdentifier(iface.Name) {
			return errors.Errorf(errors.KindValidation, "invalid interface definition name '%s'", iface.Name)
		}
	}

	// Validate IPSets
	for _, ipset := range cfg.IPSets {
		if !isValidIdentifier(ipset.Name) {
			return errors.Errorf(errors.KindValidation, "invalid ipset name '%s'", ipset.Name)
		}
	}

	// Policies are validated implicitly because they reference Zones
	// but we check the from/to fields just in case
	for _, pol := range cfg.Policies {
		if !isValidIdentifier(pol.From) {
			return errors.Errorf(errors.KindValidation, "invalid policy from-zone '%s'", pol.From)
		}
		if pol.To != "Firewall" && !isValidIdentifier(pol.To) {
			return errors.Errorf(errors.KindValidation, "invalid policy to-zone '%s'", pol.To)
		}
	}

	return nil
}

// Helper methods removed as they are now handled by atomic_apply.go script builder
// including: addRule, addSingleRule, addICMPRule, addJumpRule, addInputJumpRule,
// addBaseRules, addCtStatusRule, addProtoRule, addLoopbackRule, addCtStateRule,
// addDropRule, addPolicyDefaultRule.

// Helper methods removed: applyNAT, addMasqueradeRule, addDNATRule, addRedirectRule, applyIPSetBlockRules, addSetMatchRule

// applyIPSets creates nftables sets and populates them with IPs from config or FireHOL lists.
func (m *Manager) applyIPSets(cfg *Config, ipsetMgr *IPSetManager) error {
	if len(cfg.IPSets) == 0 {
		return nil
	}

	// Create ListManager for downloading lists
	listMgr, err := NewListManager(m.cacheDir, m.logger, "")
	if err != nil {
		m.logger.Warn("Failed to initialize ListManager (using defaults)", "error", err)
	}

	for _, ipset := range cfg.IPSets {
		// Determine set type
		setType := SetTypeIPv4Addr
		if ipset.Type != "" {
			setType = SetType(ipset.Type)
		}

		// Create the set with interval flag for CIDR support
		if err := ipsetMgr.CreateSet(ipset.Name, setType, "interval"); err != nil {
			// Set might already exist, continue
		}

		// Populate the set based on source
		var entries []string

		// Static entries from config
		if len(ipset.Entries) > 0 {
			entries = append(entries, ipset.Entries...)
		}

		// Managed List (Generic or Legacy FireHOL)
		listName := ipset.ManagedList
		if listName == "" {
			listName = ipset.FireHOLList // Fallback
		}

		if listName != "" && listMgr != nil {
			ips, err := listMgr.DownloadList(listName)
			if err != nil {
				// Log warning but continue - network might be unavailable
				m.logger.Warn("failed to download managed list", "list", listName, "error", err)
			} else {
				entries = append(entries, ips...)
				m.logger.Info("Downloaded IPs from managed list", "list", listName, "count", len(ips))
			}
		}

		// Custom URL
		if ipset.URL != "" && listMgr != nil {
			ips, err := listMgr.DownloadFromURL(ipset.URL)
			if err != nil {
				m.logger.Warn("failed to download from URL", "url", ipset.URL, "error", err)
			} else {
				entries = append(entries, ips...)
				m.logger.Info("Downloaded IPs from URL", "url", ipset.URL, "count", len(ips))
			}
		}

		// Add entries to set
		if len(entries) > 0 {
			if err := ipsetMgr.FlushSet(ipset.Name); err != nil {
				// Might fail if set is new, continue
			}
			if err := ipsetMgr.AddElements(ipset.Name, entries); err != nil {
				return fmt.Errorf("failed to add entries to set %s: %w", ipset.Name, err)
			}
			m.logger.Info("Loaded IPSet entries", "set", ipset.Name, "count", len(entries))
		}
	}

	return nil
}

// hostEndianBytes returns the uint32 bytes in the system's native (host) byte order.
// CRITICAL: This MUST ONLY be used for kernel metadata fields (like ct state, meta mark)
// which typically expect host byte order.
// DO NOT use this for packet headers (IP, Port, etc) which are always Network Byte Order (Big Endian).
func hostEndianBytes(v uint32) []byte {
	var buf [4]byte
	binary.NativeEndian.PutUint32(buf[:], v)
	return buf[:]
}

func pad(s string) []byte {
	b := make([]byte, 16)
	copy(b, s)
	return b
}

// GenerateRules generates the nftables ruleset script for the given configuration.
// It does not apply the rules.
func (m *Manager) GenerateRules(cfg *Config, resolvedIPSets map[string][]string) (string, error) {
	// Compute config hash for metadata tracking
	// Use a simple representation - zone count + policy count + interface count
	// This is fast and changes when config structure changes
	configSummary := fmt.Sprintf("z%d:p%d:i%d:n%d",
		len(cfg.Zones), len(cfg.Policies), len(cfg.Interfaces), len(cfg.NAT))
	configHash := HashConfig([]byte(configSummary))

	// 1. Build filter table script
	filterScript, err := BuildFilterTableScript(
		cfg, cfg.VPN, brand.LowerName, configHash, resolvedIPSets)
	if err != nil {
		return "", fmt.Errorf("failed to build filter table script: %w", err)
	}

	// 2. Build NAT table script (if needed)
	natScript, err := BuildNATTableScript(cfg, "nat")
	if err != nil {
		return "", fmt.Errorf("failed to build NAT table script: %w", err)
	}

	// 3. Build Mangle table script (Management Routing)
	mangleScript, err := BuildMangleTableScript(cfg, brand.LowerName)
	if err != nil {
		return "", fmt.Errorf("failed to build mangle table script: %w", err)
	}

	// 4. Combine scripts with atomic flush + rebuild
	var combinedScript strings.Builder

	// Flush entire ruleset first - this is atomic with the new rules
	// SMART FLUSH UPDATE: We no longer flush the entire ruleset to preserve dynamic sets.
	// Granular flushes are now handled by the script builders (chains, static sets).
	// combinedScript.WriteString("flush ruleset\n")

	// Add filter table (inet)
	combinedScript.WriteString(filterScript.Build())
	combinedScript.WriteString("\n")

	// Add NAT table if present
	if natScript != nil {
		combinedScript.WriteString(natScript.Build())
		combinedScript.WriteString("\n")
	}

	// Add Mangle table if present
	if mangleScript != nil {
		combinedScript.WriteString(mangleScript.Build())
		combinedScript.WriteString("\n")
	}

	return combinedScript.String(), nil
}

// PreRenderSafeMode generates and caches the safe mode ruleset for instant application.
// Call this during startup after config is loaded.
func (m *Manager) PreRenderSafeMode(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Determine trusted interfaces (non-WAN)
	var trustedInterfaces []string
	for _, iface := range cfg.Interfaces {
		// Skip WAN-like interfaces
		isWAN := false
		for _, zone := range cfg.Zones {
			if zone.Name == "WAN" || zone.Name == "wan" || zone.Name == "Internet" {
				for _, m := range zone.Matches {
					if m.Interface == iface.Name {
						isWAN = true
						break
					}
				}
				if iface.Zone == zone.Name {
					isWAN = true
				}
			}
		}
		if !isWAN {
			trustedInterfaces = append(trustedInterfaces, iface.Name)
		}
	}

	m.safeModeScript = BuildSafeModeScript(brand.LowerName, trustedInterfaces)
	m.logger.Info("Safe mode ruleset pre-rendered", "trusted_interfaces", trustedInterfaces)
}

// ApplySafeMode instantly applies the pre-rendered safe mode ruleset.
// This is the "big red button" for emergency lockdown.
// LAN can still access Web UI/API, but no forwarding occurs.
func (m *Manager) ApplySafeMode() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.safeModeScript == "" {
		return fmt.Errorf("safe mode not pre-rendered; call PreRenderSafeMode first")
	}

	applier := NewAtomicApplier()
	if err := applier.ApplyScript(m.safeModeScript); err != nil {
		return fmt.Errorf("failed to apply safe mode: %w", err)
	}

	m.inSafeMode = true
	m.logger.Warn("SAFE MODE ACTIVATED - forwarding disabled")
	return nil
}

// ExitSafeMode reapplies the normal configuration.
func (m *Manager) ExitSafeMode() error {
	m.mu.Lock()
	if m.baseConfig == nil {
		m.mu.Unlock()
		return fmt.Errorf("no base config to restore")
	}
	cfg := m.baseConfig
	m.inSafeMode = false
	m.mu.Unlock()

	m.logger.Info("Exiting safe mode, reapplying normal config")
	return m.ApplyConfig(cfg)
}

// AuthorizeIP adds an IP to the DNS egress allowlist with the specified TTL.
// This supports the "DNS Wall" feature.
func (m *Manager) AuthorizeIP(ip net.IP, ttl time.Duration) error {
	// Only proceed if Egress Filter is enabled in current config
	// (Check currentConfig, or just do best effort)
	m.mu.RLock()
	// Strict check? Or just add if set exists?
	// If feature disabled, set won't exist, and AddElements will fail silently or error.
	// We'll trust the caller (DNS Service) to only call this if configured,
	// OR we tolerate failure if set doesn't exist.
	m.mu.RUnlock()

	// Determine set name based on IP family
	setName := "dns_allowed_v4"
	if ip.To4() == nil {
		setName = "dns_allowed_v6"
	}

	// Calculate timeout in seconds
	timeout := int(ttl.Seconds())
	if timeout < 60 {
		timeout = 60 // Minimum 1 minute
	}

	// Add element with timeout using NativeIPSetManager
	// We need a transient IPSetManager instance or use the one from applyIPSets?
	// Manager usually creates IPSetManager on demand.
	ipsetMgr := NewIPSetManager(brand.LowerName)

	// Prepare element with "timeout" option
	// NativeIPSetManager.AddElements supports "element" strings.
	// For timeouts, the syntax is "1.2.3.4 timeout 300".
	element := fmt.Sprintf("%s timeout %d", ip.String(), timeout)

	if err := ipsetMgr.AddElements(setName, []string{element}); err != nil {
		// Suppress error if set doesn't exist (feature disabled)
		if strings.Contains(err.Error(), "No such file or directory") || strings.Contains(err.Error(), "does not exist") {
			return nil
		}
		return fmt.Errorf("failed to authorize IP %s: %w", ip, err)
	}

	return nil
}

// IsInSafeMode returns whether safe mode is currently active.
func (m *Manager) IsInSafeMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.inSafeMode
}

// SetIntegrityRestoreCallback sets the function to call after an integrity restore.
// This allows other services (like DNS) to re-sync their state (e.g. dynamic sets)
// after the firewall ruleset has been forcibly reverted.
func (m *Manager) SetIntegrityRestoreCallback(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restoreCallback = fn
}
