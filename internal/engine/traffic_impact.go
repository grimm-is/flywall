// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package engine

import (
	"fmt"
	"net"
	"time"

	"grimm.is/flywall/internal/config"
)

// TrafficImpactAnalyzer analyzes the impact of configuration changes on live traffic
type TrafficImpactAnalyzer struct {
	currentConfig *config.Config
	trafficStore  TrafficStore
	flowMatcher   *FlowMatcher
}

// TrafficStore stores and retrieves live traffic flows
type TrafficStore interface {
	GetRecentFlows(duration time.Duration) ([]TrafficFlow, error)
	GetActiveFlows() ([]TrafficFlow, error)
	StoreFlow(flow TrafficFlow) error
}

// TrafficFlow represents a network traffic flow
type TrafficFlow struct {
	Timestamp   time.Time
	SrcIP       net.IP
	DstIP       net.IP
	SrcPort     uint16
	DstPort     uint16
	Protocol    string
	Interface   string
	Zone        string
	Bytes       uint64
	Packets     uint64
	State       string // NEW, ESTABLISHED, RELATED, etc.
	Action      string // Current action: ACCEPT, DROP, REJECT
	MatchedRule string // Rule that matched this flow
}

// FlowMatcher matches flows against firewall rules
type FlowMatcher struct{}

// ImpactAnalysis contains the results of traffic impact analysis
type ImpactAnalysis struct {
	TotalFlows        int           `json:"total_flows"`
	AffectedFlows     int           `json:"affected_flows"`
	ChangedFlows      []FlowImpact  `json:"changed_flows"`
	NewlyBlockedFlows []TrafficFlow `json:"newly_blocked_flows"`
	NewlyAllowedFlows []TrafficFlow `json:"newly_allowed_flows"`
	Summary           ImpactSummary `json:"summary"`
	GeneratedAt       time.Time     `json:"generated_at"`
}

// FlowImpact shows how a specific flow is affected
type FlowImpact struct {
	Flow           TrafficFlow `json:"flow"`
	PreviousAction string      `json:"previous_action"`
	NewAction      string      `json:"new_action"`
	ImpactType     string      `json:"impact_type"` // BLOCKED, ALLOWED, RESTRICTED, PERMISSIVE
	RuleChange     string      `json:"rule_change"`
}

// ImpactSummary provides a high-level summary
type ImpactSummary struct {
	BlockedToAllowed int             `json:"blocked_to_allowed"`
	AllowedToBlocked int             `json:"allowed_to_blocked"`
	RestrictedMore   int             `json:"restricted_more"`
	PermissiveMore   int             `json:"permissive_more"`
	NoChange         int             `json:"no_change"`
	CriticalServices []ServiceImpact `json:"critical_services"`
}

// ServiceImpact tracks impact on critical services
type ServiceImpact struct {
	ServiceName string   `json:"service_name"`
	Protocol    string   `json:"protocol"`
	Port        uint16   `json:"port"`
	AffectedIPs []net.IP `json:"affected_ips"`
	Impact      string   `json:"impact"`
}

// NewTrafficImpactAnalyzer creates a new traffic impact analyzer
func NewTrafficImpactAnalyzer(currentConfig *config.Config, store TrafficStore) *TrafficImpactAnalyzer {
	return &TrafficImpactAnalyzer{
		currentConfig: currentConfig,
		trafficStore:  store,
		flowMatcher:   NewFlowMatcher(),
	}
}

// AnalyzeImpact analyzes the impact of a proposed configuration change
func (tia *TrafficImpactAnalyzer) AnalyzeImpact(proposedConfig *config.Config, window time.Duration) (*ImpactAnalysis, error) {
	// Get recent traffic flows
	flows, err := tia.trafficStore.GetRecentFlows(window)
	if err != nil {
		return nil, fmt.Errorf("failed to get traffic flows: %w", err)
	}

	analysis := &ImpactAnalysis{
		TotalFlows:   len(flows),
		ChangedFlows: make([]FlowImpact, 0),
		GeneratedAt:  time.Now(),
	}

	// Analyze each flow
	for _, flow := range flows {
		impact := tia.analyzeFlowImpact(flow, proposedConfig)
		if impact != nil {
			analysis.ChangedFlows = append(analysis.ChangedFlows, *impact)

			// Categorize the impact
			switch impact.ImpactType {
			case "ALLOWED_TO_BLOCKED":
				analysis.NewlyBlockedFlows = append(analysis.NewlyBlockedFlows, flow)
			case "BLOCKED_TO_ALLOWED":
				analysis.NewlyAllowedFlows = append(analysis.NewlyAllowedFlows, flow)
			}
		}
	}

	analysis.AffectedFlows = len(analysis.ChangedFlows)
	analysis.Summary = tia.generateSummary(analysis.ChangedFlows)

	return analysis, nil
}

// analyzeFlowImpact determines how a flow is affected by the config change
func (tia *TrafficImpactAnalyzer) analyzeFlowImpact(flow TrafficFlow, proposedConfig *config.Config) *FlowImpact {
	// Match flow against current config
	currentAction, currentRule := tia.flowMatcher.MatchFlow(flow, tia.currentConfig)

	// Match flow against proposed config
	newAction, newRule := tia.flowMatcher.MatchFlow(flow, proposedConfig)

	// If action hasn't changed, no impact
	if currentAction == newAction {
		return nil
	}

	impact := &FlowImpact{
		Flow:           flow,
		PreviousAction: currentAction,
		NewAction:      newAction,
		RuleChange:     fmt.Sprintf("%s -> %s", currentRule, newRule),
	}

	// Determine impact type
	switch {
	case currentAction == "ACCEPT" && newAction == "DROP":
		impact.ImpactType = "ALLOWED_TO_BLOCKED"
	case currentAction == "DROP" && newAction == "ACCEPT":
		impact.ImpactType = "BLOCKED_TO_ALLOWED"
	case currentAction == "ACCEPT" && newAction == "REJECT":
		impact.ImpactType = "ALLOWED_TO_BLOCKED"
	case currentAction == "REJECT" && newAction == "ACCEPT":
		impact.ImpactType = "BLOCKED_TO_ALLOWED"
	default:
		impact.ImpactType = "ACTION_CHANGED"
	}

	return impact
}

// generateSummary creates a summary of all impacts
func (tia *TrafficImpactAnalyzer) generateSummary(impacts []FlowImpact) ImpactSummary {
	summary := ImpactSummary{
		CriticalServices: make([]ServiceImpact, 0),
	}

	serviceMap := make(map[string]*ServiceImpact)

	for _, impact := range impacts {
		switch impact.ImpactType {
		case "ALLOWED_TO_BLOCKED":
			summary.AllowedToBlocked++
		case "BLOCKED_TO_ALLOWED":
			summary.BlockedToAllowed++
		case "RESTRICTED_MORE":
			summary.RestrictedMore++
		case "PERMISSIVE_MORE":
			summary.PermissiveMore++
		default:
			summary.NoChange++
		}

		// Track critical services (common ports)
		if tia.isCriticalService(impact.Flow.DstPort) {
			serviceKey := fmt.Sprintf("%s:%d", impact.Flow.Protocol, impact.Flow.DstPort)
			if service, exists := serviceMap[serviceKey]; exists {
				// Add IP if not already tracked
				for _, ip := range service.AffectedIPs {
					if ip.Equal(impact.Flow.DstIP) {
						service.Impact = impact.ImpactType
						break
					}
				}
				service.AffectedIPs = append(service.AffectedIPs, impact.Flow.DstIP)
			} else {
				service := &ServiceImpact{
					ServiceName: tia.getServiceName(impact.Flow.DstPort),
					Protocol:    impact.Flow.Protocol,
					Port:        impact.Flow.DstPort,
					AffectedIPs: []net.IP{impact.Flow.DstIP},
					Impact:      impact.ImpactType,
				}
				serviceMap[serviceKey] = service
				summary.CriticalServices = append(summary.CriticalServices, *service)
			}
		}
	}

	return summary
}

// isCriticalService checks if a port is a critical service
func (tia *TrafficImpactAnalyzer) isCriticalService(port uint16) bool {
	criticalPorts := []uint16{
		22,    // SSH
		53,    // DNS
		80,    // HTTP
		443,   // HTTPS
		3306,  // MySQL
		5432,  // PostgreSQL
		6379,  // Redis
		27017, // MongoDB
	}

	for _, p := range criticalPorts {
		if p == port {
			return true
		}
	}
	return false
}

// getServiceName returns the service name for a port
func (tia *TrafficImpactAnalyzer) getServiceName(port uint16) string {
	serviceNames := map[uint16]string{
		22:    "SSH",
		53:    "DNS",
		80:    "HTTP",
		443:   "HTTPS",
		3306:  "MySQL",
		5432:  "PostgreSQL",
		6379:  "Redis",
		27017: "MongoDB",
	}

	if name, exists := serviceNames[port]; exists {
		return name
	}
	return fmt.Sprintf("Port-%d", port)
}

// NewFlowMatcher creates a new flow matcher
func NewFlowMatcher() *FlowMatcher {
	return &FlowMatcher{}
}

// MatchFlow matches a traffic flow against firewall rules.
// Simplified stub until a full rule engine is implemented.
func (fm *FlowMatcher) MatchFlow(flow TrafficFlow, cfg *config.Config) (string, string) {
	if flow.DstPort == 22 {
		return "DROP", "default-drop-ssh"
	}
	if flow.DstPort == 80 || flow.DstPort == 443 {
		return "ACCEPT", "allow-web-traffic"
	}
	return "DROP", "default-policy"
}

// RealTimeImpactMonitor monitors traffic impact in real-time
type RealTimeImpactMonitor struct {
	analyzer   *TrafficImpactAnalyzer
	updateChan chan TrafficFlow
	impactChan chan FlowImpact
	stopChan   chan struct{}
}

// NewRealTimeImpactMonitor creates a new real-time monitor
func NewRealTimeImpactMonitor(analyzer *TrafficImpactAnalyzer) *RealTimeImpactMonitor {
	return &RealTimeImpactMonitor{
		analyzer:   analyzer,
		updateChan: make(chan TrafficFlow, 1000),
		impactChan: make(chan FlowImpact, 1000),
		stopChan:   make(chan struct{}),
	}
}

// Start begins real-time monitoring
func (rtim *RealTimeImpactMonitor) Start(proposedConfig *config.Config) {
	go rtim.monitorLoop(proposedConfig)
}

// Stop stops the monitoring
func (rtim *RealTimeImpactMonitor) Stop() {
	close(rtim.stopChan)
}

// monitorLoop runs the monitoring loop
func (rtim *RealTimeImpactMonitor) monitorLoop(proposedConfig *config.Config) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rtim.stopChan:
			return
		case flow := <-rtim.updateChan:
			// Analyze impact of new flow
			impact := rtim.analyzer.analyzeFlowImpact(flow, proposedConfig)
			if impact != nil {
				select {
				case rtim.impactChan <- *impact:
				default:
					// Channel full, skip
				}
			}
		case <-ticker.C:
			// Periodic analysis of active flows
			flows, err := rtim.analyzer.trafficStore.GetActiveFlows()
			if err == nil {
				for _, flow := range flows {
					impact := rtim.analyzer.analyzeFlowImpact(flow, proposedConfig)
					if impact != nil {
						select {
						case rtim.impactChan <- *impact:
						default:
							// Channel full, skip
						}
					}
				}
			}
		}
	}
}

// GetImpactChannel returns the channel for impact updates
func (rtim *RealTimeImpactMonitor) GetImpactChannel() <-chan FlowImpact {
	return rtim.impactChan
}

// UpdateFlow updates a flow in real-time
func (rtim *RealTimeImpactMonitor) UpdateFlow(flow TrafficFlow) {
	select {
	case rtim.updateChan <- flow:
	default:
		// Channel full, skip
	}
}
