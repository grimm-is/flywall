package main

import (
	"errors"
	"time"

	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/analytics"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
	"grimm.is/flywall/internal/firewall"
	"grimm.is/flywall/internal/identity"
	"grimm.is/flywall/internal/kernel"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/learning/flowdb"
	"grimm.is/flywall/internal/metrics"
	"grimm.is/flywall/internal/services/dns/querylog"
	"grimm.is/flywall/internal/services/scanner"
)

// SimControlPlaneClient implements ctlplane.ControlPlaneClient for the simulator.
// It bridges the API server to the SimKernel and LearningEngine.
type SimControlPlaneClient struct {
	config *config.Config
	kernel *kernel.SimKernel
	engine *learning.Engine
}

// NewSimControlPlaneClient creates a new simulator client.
func NewSimControlPlaneClient(cfg *config.Config, k *kernel.SimKernel, e *learning.Engine) *SimControlPlaneClient {
	// Initialize kernel config
	if cfg != nil {
		k.LoadConfig(cfg)
	}
	return &SimControlPlaneClient{
		config: cfg,
		kernel: k,
		engine: e,
	}
}

// Close closes the connection (no-op for sim)
func (c *SimControlPlaneClient) Close() error {
	return nil
}

// --- Status & Config ---

func (c *SimControlPlaneClient) GetReplicationStatus() (*ctlplane.GetReplicationStatusReply, error) {
	return &ctlplane.GetReplicationStatusReply{
		Status: ctlplane.ReplicationStatus{
			Mode:  "simulated",
			Error: "",
		},
	}, nil
}

func (c *SimControlPlaneClient) GetStatus() (*ctlplane.Status, error) {
	return &ctlplane.Status{
		Running:        true,
		Uptime:         time.Since(time.Now()).String(),
		ConfigFile:     "sim_config.hcl",
		FirewallActive: true,
	}, nil
}

func (c *SimControlPlaneClient) GetConfig() (*config.Config, error) {
	return c.config, nil
}

func (c *SimControlPlaneClient) GetInterfaces() ([]ctlplane.InterfaceStatus, error) {
	// Return simulated interfaces
	return []ctlplane.InterfaceStatus{
		{Name: "wan0", Type: "ethernet", State: ctlplane.InterfaceStateUp, IPv4Addrs: []string{"192.0.2.1/24"}},
		{Name: "lan0", Type: "ethernet", State: ctlplane.InterfaceStateUp, IPv4Addrs: []string{"10.0.0.1/24"}},
		{Name: "mgmt0", Type: "ethernet", State: ctlplane.InterfaceStateUp, IPv4Addrs: []string{"127.0.0.1/8"}},
	}, nil
}

func (c *SimControlPlaneClient) GetServices() ([]ctlplane.ServiceStatus, error) {
	return []ctlplane.ServiceStatus{
		{Name: "firewall", Running: true},
		{Name: "dhcp", Running: true},
		{Name: "dns", Running: true},
		{Name: "learning", Running: true},
	}, nil
}

func (c *SimControlPlaneClient) ApplyConfig(cfg *config.Config) error {
	c.config = cfg
	c.kernel.LoadConfig(cfg)
	return nil
}

func (c *SimControlPlaneClient) RestartService(serviceName string) error {
	return nil
}

func (c *SimControlPlaneClient) Reboot() error {
	return errors.New("cannot reboot simulator")
}

func (c *SimControlPlaneClient) GetDHCPLeases() ([]ctlplane.DHCPLease, error) {
	// Return mock leases or extract from Replayer state if passed
	return []ctlplane.DHCPLease{}, nil
}

// --- Learning Firewall ---

func (c *SimControlPlaneClient) GetLearningRules(status string) ([]*learning.PendingRule, error) {
	// Not implemented in engine yet? Engine deals with Flows, not Rules directly?
	// The interface uses PendingRule.
	// Actually, Engine has GetPendingFlows which returns FlowWithHints.
	// We might need to map Flows to PendingRules if API expects it.
	return []*learning.PendingRule{}, nil
}

func (c *SimControlPlaneClient) GetLearningRule(id string) (*learning.PendingRule, error) {
	return nil, errors.New("not implemented")
}

func (c *SimControlPlaneClient) ApproveRule(id, user string) (*learning.PendingRule, error) {
	return nil, errors.New("not implemented")
}

func (c *SimControlPlaneClient) DenyRule(id, user string) (*learning.PendingRule, error) {
	return nil, errors.New("not implemented")
}

func (c *SimControlPlaneClient) IgnoreRule(id string) (*learning.PendingRule, error) {
	return nil, errors.New("not implemented")
}

func (c *SimControlPlaneClient) DeleteRule(id string) error {
	return errors.New("not implemented")
}

func (c *SimControlPlaneClient) GetLearningStats() (map[string]interface{}, error) {
	if c.engine != nil {
		stats, err := c.engine.GetStats()
		if err != nil {
			return nil, err
		}
		// Convert map[string]int64 to map[string]interface{}
		res := make(map[string]interface{})
		for k, v := range stats {
			res[k] = v
		}
		return res, nil
	}
	return nil, nil
}

func (c *SimControlPlaneClient) GetTopology() (*ctlplane.GetTopologyReply, error) {
	return &ctlplane.GetTopologyReply{}, nil
}

func (c *SimControlPlaneClient) GetNetworkDevices() ([]ctlplane.NetworkDevice, error) {
	return []ctlplane.NetworkDevice{}, nil
}

// --- Flow Management ---

func (c *SimControlPlaneClient) GetFlows(state string, limit, offset int) ([]flowdb.FlowWithHints, map[string]int64, error) {
	if c.engine != nil {
		flows, err := c.engine.ListFlows(flowdb.ListOptions{
			State:  flowdb.FlowState(state),
			Limit:  limit,
			Offset: offset,
			Desc:   true,
		})
		if err != nil {
			return nil, nil, err
		}
		// Stats needs to be fetched separately if needed, but signature returns map
		stats, _ := c.engine.GetStats()
		return flows, stats, nil
	}
	return nil, nil, nil
}

func (c *SimControlPlaneClient) ApproveFlow(id int64) error {
	if c.engine != nil {
		return c.engine.AllowFlow(id)
	}
	return nil
}

func (c *SimControlPlaneClient) DenyFlow(id int64) error {
	if c.engine != nil {
		return c.engine.DenyFlow(id)
	}
	return nil
}

func (c *SimControlPlaneClient) DeleteFlow(id int64) error {
	if c.engine != nil {
		return c.engine.DeleteFlow(id)
	}
	return nil
}

// --- Stubs for everything else ---

func (c *SimControlPlaneClient) GetAvailableInterfaces() ([]ctlplane.AvailableInterface, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) UpdateInterface(args *ctlplane.UpdateInterfaceArgs) (*ctlplane.UpdateInterfaceReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) CreateVLAN(args *ctlplane.CreateVLANArgs) (*ctlplane.CreateVLANReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) DeleteVLAN(ifaceName string) (*ctlplane.UpdateInterfaceReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) CreateBond(args *ctlplane.CreateBondArgs) (*ctlplane.CreateBondReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) DeleteBond(name string) (*ctlplane.UpdateInterfaceReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetRawHCL() (*ctlplane.GetRawHCLReply, error) { return nil, nil }
func (c *SimControlPlaneClient) GetSectionHCL(sectionType string, labels ...string) (*ctlplane.GetSectionHCLReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) SetRawHCL(hcl string) (*ctlplane.SetRawHCLReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) SetSectionHCL(sectionType string, hcl string, labels ...string) (*ctlplane.SetSectionHCLReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) DeleteSection(sectionType string) (*ctlplane.DeleteSectionReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) DeleteSectionByLabel(sectionType string, labels ...string) (*ctlplane.DeleteSectionReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) ValidateHCL(hcl string) (*ctlplane.ValidateHCLReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) SaveConfig() (*ctlplane.SaveConfigReply, error)   { return nil, nil }
func (c *SimControlPlaneClient) GetConfigDiff() (string, error)                   { return "", nil }
func (c *SimControlPlaneClient) ListBackups() (*ctlplane.ListBackupsReply, error) { return nil, nil }
func (c *SimControlPlaneClient) Upgrade(checksum string) error                    { return nil }
func (c *SimControlPlaneClient) StageBinary(data []byte, checksum, arch string) (*ctlplane.StageBinaryReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) CreateBackup(description string, pinned bool) (*ctlplane.CreateBackupReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) RestoreBackup(version int) (*ctlplane.RestoreBackupReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetBackupContent(version int) (*ctlplane.GetBackupContentReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) PinBackup(version int, pinned bool) (*ctlplane.PinBackupReply, error) {
	return nil, nil
}

// --- Device Identity Management ---

func (c *SimControlPlaneClient) UpdateDeviceIdentity(args *ctlplane.UpdateDeviceIdentityArgs) (*identity.DeviceIdentity, error) {
	alias := ""
	if args.Alias != nil {
		alias = *args.Alias
	}
	owner := ""
	if args.Owner != nil {
		owner = *args.Owner
	}
	return &identity.DeviceIdentity{
		ID:        args.MAC,
		MACs:      []string{args.MAC},
		Alias:     alias,
		Owner:     owner,
		Tags:      args.Tags,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}, nil
}
func (c *SimControlPlaneClient) GetDeviceGroups() ([]identity.DeviceGroup, error)   { return nil, nil }
func (c *SimControlPlaneClient) UpdateDeviceGroup(group identity.DeviceGroup) error { return nil }
func (c *SimControlPlaneClient) DeleteDeviceGroup(id string) error                  { return nil }
func (c *SimControlPlaneClient) SetMaxBackups(maxBackups int) (*ctlplane.SetMaxBackupsReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetLogs(args *ctlplane.GetLogsArgs) (*ctlplane.GetLogsReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetLogSources() (*ctlplane.GetLogSourcesReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetLogStats() (*ctlplane.GetLogStatsReply, error) { return nil, nil }
func (c *SimControlPlaneClient) TriggerTask(taskName string) error                { return nil }
func (c *SimControlPlaneClient) SafeApplyInterface(args *ctlplane.SafeApplyInterfaceArgs) (*firewall.ApplyResult, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) ConfirmApplyInterface(applyID string) error    { return nil }
func (c *SimControlPlaneClient) CancelApplyInterface(applyID string) error     { return nil }
func (c *SimControlPlaneClient) ListIPSets() ([]firewall.IPSetMetadata, error) { return nil, nil }
func (c *SimControlPlaneClient) GetIPSet(name string) (*firewall.IPSetMetadata, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) RefreshIPSet(name string) error                     { return nil }
func (c *SimControlPlaneClient) GetIPSetElements(name string) ([]string, error)     { return nil, nil }
func (c *SimControlPlaneClient) GetIPSetCacheInfo() (map[string]interface{}, error) { return nil, nil }
func (c *SimControlPlaneClient) ClearIPSetCache() error                             { return nil }
func (c *SimControlPlaneClient) AddToIPSet(name, ip string) error                   { return nil }
func (c *SimControlPlaneClient) RemoveFromIPSet(name, ip string) error              { return nil }
func (c *SimControlPlaneClient) CheckIPSet(name, ip string) (bool, error)           { return false, nil }
func (c *SimControlPlaneClient) SystemReboot(force bool) (string, error)            { return "", nil }
func (c *SimControlPlaneClient) GetSystemStats() (*ctlplane.SystemStats, error) {
	return &ctlplane.SystemStats{}, nil
}
func (c *SimControlPlaneClient) GetRoutes() ([]ctlplane.Route, error) { return nil, nil }
func (c *SimControlPlaneClient) GetNotifications(sinceID int64) ([]ctlplane.Notification, int64, error) {
	return nil, 0, nil
}
func (c *SimControlPlaneClient) GetUplinkGroups() ([]ctlplane.UplinkGroupStatus, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) SwitchUplink(groupName, uplinkName string) error { return nil }
func (c *SimControlPlaneClient) ToggleUplink(groupName, uplinkName string, enabled bool) error {
	return nil
}
func (c *SimControlPlaneClient) TestUplink(groupName, uplinkName string) (*ctlplane.UplinkStatus, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetPolicyStats() (map[string]*metrics.PolicyStats, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) StartScanNetwork(cidr string, timeoutSeconds int) error { return nil }
func (c *SimControlPlaneClient) GetScanStatus() (bool, *scanner.ScanResult, error) {
	return false, nil, nil
}
func (c *SimControlPlaneClient) GetScanResult() (*scanner.ScanResult, error)     { return nil, nil }
func (c *SimControlPlaneClient) GetCommonPorts() ([]scanner.Port, error)         { return nil, nil }
func (c *SimControlPlaneClient) ScanHost(ip string) (*scanner.HostResult, error) { return nil, nil }
func (c *SimControlPlaneClient) WakeOnLAN(mac, iface string) error               { return nil }
func (c *SimControlPlaneClient) LinkMAC(mac, identityID string) error            { return nil }
func (c *SimControlPlaneClient) UnlinkMAC(mac string) error                      { return nil }
func (c *SimControlPlaneClient) Ping(target string, timeoutSeconds int) (*ctlplane.PingReply, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) IsInSafeMode() (bool, error) { return false, nil }
func (c *SimControlPlaneClient) EnterSafeMode() error        { return nil }
func (c *SimControlPlaneClient) ExitSafeMode() error         { return nil }

// --- Analytics ---
func (c *SimControlPlaneClient) GetAnalyticsBandwidth(args *ctlplane.GetAnalyticsBandwidthArgs) ([]ctlplane.BandwidthPoint, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetAnalyticsTopTalkers(args *ctlplane.GetAnalyticsTopTalkersArgs) ([]analytics.Summary, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetAnalyticsFlows(args *ctlplane.GetAnalyticsFlowsArgs) ([]analytics.Summary, error) {
	return nil, nil
}

// --- Alerting ---
func (c *SimControlPlaneClient) GetAlertHistory(limit int) ([]alerting.AlertEvent, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetAlertRules() ([]alerting.AlertRule, error) {
	return nil, nil
}

func (c *SimControlPlaneClient) UpdateAlertRule(rule alerting.AlertRule) error {
	return nil
}

// --- DNS Query Log ---
func (c *SimControlPlaneClient) GetDNSQueryHistory(limit, offset int, search string) ([]querylog.Entry, error) {
	return nil, nil
}
func (c *SimControlPlaneClient) GetDNSStats(from, to time.Time) (*querylog.Stats, error) {
	return &querylog.Stats{}, nil
}
