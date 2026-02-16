// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package engine

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/firewall"
)

// IntegratedEngine combines validation, simulation, and firewall generation
type IntegratedEngine struct {
	configValidator *config.IntentValidator
	firewallBuilder *firewall.ScriptBuilder
	trafficAnalyzer *TrafficImpactAnalyzer
}

// NewIntegratedEngine creates a new integrated engine instance
func NewIntegratedEngine() *IntegratedEngine {
	return &IntegratedEngine{
		firewallBuilder: firewall.NewScriptBuilder("flywall", "inet", "UTC"),
	}
}

// ValidateAndSimulate performs comprehensive validation and simulation
func (ie *IntegratedEngine) ValidateAndSimulate(cfg *config.Config) (*ValidationResult, error) {
	result := &ValidationResult{
		Config:   cfg,
		Errors:   make([]error, 0),
		Warnings: make([]string, 0),
	}

	// Set config for validators
	ie.configValidator = config.NewIntentValidator(cfg)

	// 1. Syntax validation
	syntaxErrors := cfg.Validate()
	for _, err := range syntaxErrors {
		result.Errors = append(result.Errors, err)
	}

	// 2. Intent validation
	intentErrors := ie.configValidator.ValidateIntent()
	for _, err := range intentErrors {
		result.Errors = append(result.Errors, err)
	}

	// 3. Deep validation
	deepErrors := cfg.DeepValidate()
	for _, err := range deepErrors {
		result.Errors = append(result.Errors, err)
	}

	return result, nil
}

// ValidationResult contains comprehensive validation results
type ValidationResult struct {
	Config            *config.Config
	Errors            []error
	Warnings          []string
	SimulationResults []SimulationResult
	ComplianceReport  *config.ComplianceReport
	DependencyGraph   *config.DependencyGraph
}

// IsValid returns true if there are no validation errors
func (vr *ValidationResult) IsValid() bool {
	return len(vr.Errors) == 0
}

// SimulationResult represents a simulation scenario outcome
type SimulationResult struct {
	Scenario string
	Passed   bool
	Warning  bool
	Message  string
	Details  map[string]interface{}
}

// simulateCriticalScenarios runs critical connectivity simulations.
// Stub: returns empty results until simulation feature is implemented.
func (ie *IntegratedEngine) simulateCriticalScenarios(cfg *config.Config) []SimulationResult {
	return []SimulationResult{}
}

// ApplyWithValidation applies configuration with full validation
func (ie *IntegratedEngine) ApplyWithValidation(cfg *config.Config) error {
	// Validate first
	validationResult, err := ie.ValidateAndSimulate(cfg)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if len(validationResult.Errors) > 0 {
		return fmt.Errorf("configuration has %d validation errors", len(validationResult.Errors))
	}

	// Generate firewall script
	script, err := ie.GenerateFirewallScript(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate firewall script: %w", err)
	}

	// Apply the script
	return ie.applyFirewallScript(script)
}

// GenerateFirewallScript creates an optimized firewall script
func (ie *IntegratedEngine) GenerateFirewallScript(cfg *config.Config) (string, error) {
	ie.firewallBuilder.SetOptimizationEnabled(true)
	script := ie.firewallBuilder.Build()
	return script, nil
}

// applyFirewallScript applies the nftables script to the system
func (ie *IntegratedEngine) applyFirewallScript(script string) error {
	// Apply the nftables script
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply firewall rules: %w, output: %s", err, string(output))
	}
	return nil
}

// ValidateConfigOnly performs validation without applying changes
func (ie *IntegratedEngine) ValidateConfigOnly(cfg *config.Config) (*ValidationResult, error) {
	return ie.ValidateAndSimulate(cfg)
}

// SimulateConnectivity simulates network connectivity between zones.
// Stub: not yet implemented.
func (ie *IntegratedEngine) SimulateConnectivity(cfg *config.Config, from, to, protocol string, port int) (*config.ConnectivityTestResult, error) {
	return nil, fmt.Errorf("simulation not yet implemented")
}

// CheckCompliance runs compliance checks against the configuration.
// Stub: not yet implemented.
func (ie *IntegratedEngine) CheckCompliance(cfg *config.Config, policyName string) (*config.ComplianceReport, error) {
	return nil, fmt.Errorf("compliance checking not yet implemented")
}

// AnalyzeDependencies analyzes configuration dependencies.
// Stub: not yet implemented.
func (ie *IntegratedEngine) AnalyzeDependencies(cfg *config.Config) (*config.DependencyGraph, error) {
	return nil, fmt.Errorf("dependency analysis not yet implemented")
}

// DryRun performs a dry run of configuration changes
func (ie *IntegratedEngine) DryRun(cfg *config.Config) (*DryRunResult, error) {
	// Validate configuration
	validationResult, err := ie.ValidateAndSimulate(cfg)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate script without applying
	script, err := ie.GenerateFirewallScript(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate firewall script: %w", err)
	}

	return &DryRunResult{
		ValidationResult: validationResult,
		GeneratedScript:  script,
		Summary:          ie.generateSummary(validationResult, script),
	}, nil
}

// DryRunResult contains the results of a dry run
type DryRunResult struct {
	ValidationResult *ValidationResult
	GeneratedScript  string
	Summary          DryRunSummary
}

// DryRunSummary provides a summary of the dry run
type DryRunSummary struct {
	TotalRules       int
	OptimizedRules   int
	ReductionPercent float64
	HasErrors        bool
	HasWarnings      bool
	ComplianceScore  float64
}

// generateSummary creates a summary of the dry run results
func (ie *IntegratedEngine) generateSummary(result *ValidationResult, script string) DryRunSummary {
	summary := DryRunSummary{
		HasErrors:   len(result.Errors) > 0,
		HasWarnings: len(result.Warnings) > 0,
	}

	// Count lines in script (rough estimate of rules)
	lines := strings.Split(script, "\n")
	ruleCount := 0
	for _, line := range lines {
		if strings.Contains(line, "accept") || strings.Contains(line, "drop") || strings.Contains(line, "reject") {
			ruleCount++
		}
	}
	summary.TotalRules = ruleCount

	return summary
}

// AnalyzeTrafficImpact analyzes the impact of configuration changes on live traffic.
// Uses mock traffic store until real traffic store integration is available.
func (ie *IntegratedEngine) AnalyzeTrafficImpact(currentConfig, proposedConfig *config.Config, window time.Duration) (*ImpactAnalysis, error) {
	store := NewMockTrafficStore(CreateTestFlows())
	analyzer := NewTrafficImpactAnalyzer(currentConfig, store)
	return analyzer.AnalyzeImpact(proposedConfig, window)
}

// StartRealTimeMonitoring starts real-time traffic impact monitoring.
// Uses mock traffic store until real traffic store integration is available.
func (ie *IntegratedEngine) StartRealTimeMonitoring(currentConfig, proposedConfig *config.Config) (*RealTimeImpactMonitor, error) {
	store := NewMockTrafficStore(CreateTestFlows())
	analyzer := NewTrafficImpactAnalyzer(currentConfig, store)
	monitor := NewRealTimeImpactMonitor(analyzer)
	monitor.Start(proposedConfig)
	return monitor, nil
}

// GenerateTrafficReport generates a detailed traffic impact report
func (ie *IntegratedEngine) GenerateTrafficReport(analysis *ImpactAnalysis) string {
	report := strings.Builder{}

	report.WriteString("# Traffic Impact Analysis Report\n\n")
	report.WriteString(fmt.Sprintf("Generated at: %s\n", analysis.GeneratedAt.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Analysis window: Last flows\n\n"))

	// Summary section
	report.WriteString("## Summary\n\n")
	report.WriteString(fmt.Sprintf("- Total flows analyzed: %d\n", analysis.TotalFlows))
	report.WriteString(fmt.Sprintf("- Affected flows: %d (%.1f%%)\n",
		analysis.AffectedFlows,
		float64(analysis.AffectedFlows)/float64(analysis.TotalFlows)*100))

	report.WriteString(fmt.Sprintf("- Newly blocked: %d\n", len(analysis.NewlyBlockedFlows)))
	report.WriteString(fmt.Sprintf("- Newly allowed: %d\n", len(analysis.NewlyAllowedFlows)))

	// Impact breakdown
	report.WriteString("\n## Impact Breakdown\n\n")
	report.WriteString(fmt.Sprintf("- Allowed → Blocked: %d\n", analysis.Summary.AllowedToBlocked))
	report.WriteString(fmt.Sprintf("- Blocked → Allowed: %d\n", analysis.Summary.BlockedToAllowed))
	report.WriteString(fmt.Sprintf("- More restrictive: %d\n", analysis.Summary.RestrictedMore))
	report.WriteString(fmt.Sprintf("- More permissive: %d\n", analysis.Summary.PermissiveMore))

	// Critical services
	if len(analysis.Summary.CriticalServices) > 0 {
		report.WriteString("\n## Critical Services Affected\n\n")
		for _, service := range analysis.Summary.CriticalServices {
			report.WriteString(fmt.Sprintf("- **%s** (%s/%d): %s\n",
				service.ServiceName, service.Protocol, service.Port, service.Impact))
			report.WriteString(fmt.Sprintf("  - Affected IPs: %d\n", len(service.AffectedIPs)))
		}
	}

	// Detailed flow changes
	if len(analysis.ChangedFlows) > 0 {
		report.WriteString("\n## Detailed Flow Changes\n\n")
		for _, change := range analysis.ChangedFlows {
			report.WriteString(fmt.Sprintf("### Flow: %s:%d → %s:%d (%s)\n",
				change.Flow.SrcIP, change.Flow.SrcPort,
				change.Flow.DstIP, change.Flow.DstPort,
				change.Flow.Protocol))
			report.WriteString(fmt.Sprintf("- Previous action: **%s**\n", change.PreviousAction))
			report.WriteString(fmt.Sprintf("- New action: **%s**\n", change.NewAction))
			report.WriteString(fmt.Sprintf("- Impact: %s\n", change.ImpactType))
			report.WriteString(fmt.Sprintf("- Bytes transferred: %d\n", change.Flow.Bytes))
			report.WriteString(fmt.Sprintf("- Packets: %d\n", change.Flow.Packets))
			report.WriteString("\n")
		}
	}

	// Newly blocked flows
	if len(analysis.NewlyBlockedFlows) > 0 {
		report.WriteString("\n## Newly Blocked Flows\n\n")
		for _, flow := range analysis.NewlyBlockedFlows {
			report.WriteString(fmt.Sprintf("- %s:%d → %s:%d (%s) - %d bytes\n",
				flow.SrcIP, flow.SrcPort, flow.DstIP, flow.DstPort,
				flow.Protocol, flow.Bytes))
		}
	}

	// Newly allowed flows
	if len(analysis.NewlyAllowedFlows) > 0 {
		report.WriteString("\n## Newly Allowed Flows\n\n")
		for _, flow := range analysis.NewlyAllowedFlows {
			report.WriteString(fmt.Sprintf("- %s:%d → %s:%d (%s) - %d bytes\n",
				flow.SrcIP, flow.SrcPort, flow.DstIP, flow.DstPort,
				flow.Protocol, flow.Bytes))
		}
	}

	return report.String()
}
