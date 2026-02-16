// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/engine"
)

// RunConfigValidate runs comprehensive configuration validation
func RunConfigValidate(configPath string, options ValidateOptions) error {
	// Load config
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate with integrated engine
	integratedEngine := engine.NewIntegratedEngine()
	result, err := integratedEngine.ValidateAndSimulate(cfg)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Print results
	if len(result.Errors) > 0 {
		fmt.Printf("❌ Configuration validation failed with %d errors:\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err.Error())
		}
		if !options.Force {
			return fmt.Errorf("validation failed")
		}
		fmt.Printf("\n⚠️  Proceeding despite errors due to --force flag\n")
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n⚠️  Configuration has %d warnings:\n", len(result.Warnings))
		for _, warn := range result.Warnings {
			fmt.Printf("  - %s\n", warn)
		}
	}

	fmt.Printf("\n✅ Configuration validation completed!\n")

	// Print simulation results
	if len(result.SimulationResults) > 0 {
		fmt.Printf("\n📊 Simulation Results:\n")
		for _, sim := range result.SimulationResults {
			status := "✅"
			if sim.Warning {
				status = "⚠️ "
			} else if !sim.Passed {
				status = "❌"
			}
			fmt.Printf("  %s %s: %s\n", status, sim.Scenario, sim.Message)
		}
	}

	// Print compliance report
	if result.ComplianceReport != nil && options.Verbose {
		fmt.Printf("\n🛡️  Compliance Report:\n")
		fmt.Printf("  Score: %.1f%%\n", result.ComplianceReport.Score)
		fmt.Printf("  Critical: %d, High: %d, Medium: %d, Low: %d\n",
			result.ComplianceReport.CriticalCount,
			result.ComplianceReport.HighCount,
			result.ComplianceReport.MediumCount,
			result.ComplianceReport.LowCount)
	}

	return nil
}

// RunConfigSimulate runs network connectivity simulation
func RunConfigSimulate(configPath string, scenarios []string, options SimulateOptions) error {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	integratedEngine := engine.NewIntegratedEngine()

	if len(scenarios) == 0 {
		// Run default critical scenarios
		fmt.Printf("Running default critical scenarios...\n")
		result, err := integratedEngine.ValidateAndSimulate(cfg)
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}

		for _, sim := range result.SimulationResults {
			status := "ALLOWED"
			if !sim.Passed {
				status = "BLOCKED"
			}
			fmt.Printf("%-30s %s\n", sim.Scenario, status)
			if options.Verbose {
				fmt.Printf("  %s\n", sim.Message)
			}
		}
		return nil
	}

	// Run specified scenarios
	for _, scenario := range scenarios {
		err := parseAndSimulate(integratedEngine, cfg, scenario)
		if err != nil {
			fmt.Printf("❌ %s: %v\n", scenario, err)
			continue
		}

		fmt.Printf("%-40s %s\n", scenario, "SIMULATED")
		if options.Verbose {
			fmt.Printf("  Simulation completed successfully\n")
		}
	}

	return nil
}

// RunConfigPipeline runs the configuration validation pipeline
func RunConfigPipeline(configPath string, options PipelineOptions) error {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	integratedEngine := engine.NewIntegratedEngine()
	pipeline := engine.NewConfigPipeline(integratedEngine)

	// Add custom stages if provided
	for _, stage := range options.Stages {
		pipeline.AddStage(engine.ConfigStage{
			Name:        stage.Name,
			Description: stage.Description,
			Optional:    stage.Optional,
		})
	}

	ctx := context.Background()
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	result, err := pipeline.Execute(ctx, cfg)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Print results
	fmt.Printf("\nPipeline Execution Results (%.2fs):\n", result.Duration.Seconds())
	fmt.Printf("Overall Success: %t\n", result.OverallSuccess)
	fmt.Printf("Total Errors: %d\n", result.TotalErrors)
	fmt.Printf("Total Warnings: %d\n", result.TotalWarnings)

	fmt.Printf("\nStage Results:\n")
	for _, stage := range pipeline.GetStages() {
		if stageResult, exists := result.StageResults[stage.Name]; exists {
			status := "✅"
			if !stageResult.Success {
				if stage.Optional {
					status = "⚠️"
				} else {
					status = "❌"
				}
			}
			fmt.Printf("  %s %s (%.2fs)", status, stage.Name, stageResult.Duration.Seconds())
			if stageResult.Error != nil {
				fmt.Printf("\n    Error: %s", stageResult.Error.Error())
			}
			fmt.Printf("\n")
			if options.Verbose && stage.Description != "" {
				fmt.Printf("    %s\n", stage.Description)
			}
		}
	}

	return nil
}

// RunConfigDryRun performs a dry run of configuration changes
func RunConfigDryRun(configPath string, options DryRunOptions) error {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	integratedEngine := engine.NewIntegratedEngine()
	result, err := integratedEngine.DryRun(cfg)
	if err != nil {
		return fmt.Errorf("dry run failed: %w", err)
	}

	fmt.Printf("Dry Run Results:\n")
	fmt.Printf("Validation: %t\n", result.ValidationResult.IsValid())
	fmt.Printf("Total Rules: %d\n", result.Summary.TotalRules)
	fmt.Printf("Optimized Rules: %d\n", result.Summary.OptimizedRules)

	if result.Summary.ReductionPercent > 0 {
		fmt.Printf("Rule Reduction: %.1f%%\n", result.Summary.ReductionPercent)
	}

	if result.Summary.HasErrors {
		fmt.Printf("\n❌ Validation Errors:\n")
		for _, err := range result.ValidationResult.Errors {
			fmt.Printf("  - %s\n", err.Error())
		}
	}

	if result.Summary.HasWarnings {
		fmt.Printf("\n⚠️  Warnings:\n")
		for _, warn := range result.ValidationResult.Warnings {
			fmt.Printf("  - %s\n", warn)
		}
	}

	if options.OutputScript {
		fmt.Printf("\nGenerated Script:\n")
		fmt.Printf("---\n%s\n---\n", result.GeneratedScript)
	}

	if options.OutputFile != "" {
		err = os.WriteFile(options.OutputFile, []byte(result.GeneratedScript), 0644)
		if err != nil {
			return fmt.Errorf("failed to write script to file: %w", err)
		}
		fmt.Printf("\nScript saved to: %s\n", options.OutputFile)
	}

	return nil
}

// RunConfigCompliance checks configuration compliance
func RunConfigCompliance(configPath, policyName string, options ComplianceOptions) error {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if policyName == "" {
		policyName = "basic-security"
	}

	integratedEngine := engine.NewIntegratedEngine()
	report, err := integratedEngine.CheckCompliance(cfg, policyName)
	if err != nil {
		return fmt.Errorf("compliance check failed: %w", err)
	}

	fmt.Printf("Compliance Report for policy: %s\n", policyName)
	fmt.Printf("Overall Score: %.1f%%\n", report.Score)
	fmt.Printf("Critical: %d, High: %d, Medium: %d, Low: %d, Info: %d\n",
		report.CriticalCount,
		report.HighCount,
		report.MediumCount,
		report.LowCount,
		report.InfoCount)

	if options.Verbose {
		fmt.Printf("\nDetailed Results:\n")
		for _, result := range report.Results {
			status := "✅"
			if !result.Passed {
				status = "❌"
			}
			fmt.Printf("  %s [%s] %s\n", status, result.RuleID, result.Message)
			if result.Description != "" {
				fmt.Printf("    %s\n", result.Description)
			}
		}
	}

	return nil
}

// parseAndSimulate parses scenario string and runs simulation
func parseAndSimulate(engine *engine.IntegratedEngine, cfg *config.Config, scenario string) error {
	// Parse scenario: "from->to:protocol:port"
	parts := strings.Split(scenario, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid scenario format, use: from->to:protocol:port")
	}

	fromTo := strings.Split(parts[0], "->")
	if len(fromTo) != 2 {
		return fmt.Errorf("invalid scenario format, use: from->to:protocol:port")
	}

	port, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	_, err = engine.SimulateConnectivity(cfg, fromTo[0], fromTo[1], parts[1], port)
	return err
}

// Command options
type ValidateOptions struct {
	Force   bool
	Verbose bool
}

type SimulateOptions struct {
	Verbose bool
}

type PipelineOptions struct {
	Timeout time.Duration
	Stages  []CustomStage
	Verbose bool
}

type DryRunOptions struct {
	OutputScript bool
	OutputFile   string
}

type ComplianceOptions struct {
	Verbose bool
}

type CustomStage struct {
	Name        string
	Description string
	Optional    bool
}

// RunConfigApply applies configuration with validation
func RunConfigApply(configPath string, options ApplyOptions) error {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	integratedEngine := engine.NewIntegratedEngine()

	if options.DryRun {
		fmt.Printf("Performing dry run...\n")
		result, err := integratedEngine.DryRun(cfg)
		if err != nil {
			return fmt.Errorf("dry run failed: %w", err)
		}

		fmt.Printf("Dry run completed successfully\n")
		fmt.Printf("Rules to be applied: %d\n", result.Summary.TotalRules)
		return nil
	}

	// Validate before applying
	fmt.Printf("Validating configuration...\n")
	result, err := integratedEngine.ValidateAndSimulate(cfg)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if !result.IsValid() && !options.Force {
		fmt.Printf("❌ Configuration validation failed:\n")
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err.Error())
		}
		return fmt.Errorf("use --force to apply despite validation errors")
	}

	// Apply configuration
	fmt.Printf("Applying configuration...\n")
	if err := integratedEngine.ApplyWithValidation(cfg); err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}

	fmt.Printf("✅ Configuration applied successfully!\n")
	return nil
}

// ApplyOptions contains options for config apply
type ApplyOptions struct {
	DryRun bool
	Force  bool
}

// RunConfigTrafficImpact analyzes traffic impact of configuration changes
func RunConfigTrafficImpact(currentConfigPath, proposedConfigPath string, options TrafficImpactOptions) error {
	// Load configurations
	currentCfg, err := config.LoadFile(currentConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load current config: %w", err)
	}

	proposedCfg, err := config.LoadFile(proposedConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load proposed config: %w", err)
	}

	integratedEngine := engine.NewIntegratedEngine()
	window := time.Duration(options.WindowMinutes) * time.Minute

	// Analyze traffic impact
	analysis, err := integratedEngine.AnalyzeTrafficImpact(currentCfg, proposedCfg, window)
	if err != nil {
		return fmt.Errorf("traffic impact analysis failed: %w", err)
	}

	// Print results
	fmt.Printf("\n🔍 Traffic Impact Analysis\n")
	fmt.Printf("========================\n\n")
	fmt.Printf("Analysis window: Last %d minutes\n", options.WindowMinutes)
	fmt.Printf("Total flows analyzed: %d\n", analysis.TotalFlows)
	fmt.Printf("Affected flows: %d (%.1f%%)\n\n",
		analysis.AffectedFlows,
		float64(analysis.AffectedFlows)/float64(analysis.TotalFlows)*100)

	if analysis.Summary.AllowedToBlocked > 0 {
		fmt.Printf("⚠️  Flows that will be BLOCKED: %d\n", analysis.Summary.AllowedToBlocked)
	}
	if analysis.Summary.BlockedToAllowed > 0 {
		fmt.Printf("✅ Flows that will be ALLOWED: %d\n", analysis.Summary.BlockedToAllowed)
	}

	// Show critical services
	if len(analysis.Summary.CriticalServices) > 0 {
		fmt.Printf("\n🚨 Critical Services Affected:\n")
		for _, service := range analysis.Summary.CriticalServices {
			fmt.Printf("  - %s (%s/%d): %s\n",
				service.ServiceName, service.Protocol, service.Port, service.Impact)
		}
	}

	// Show detailed changes if requested
	if options.Verbose {
		fmt.Printf("\n📋 Detailed Flow Changes:\n")
		for _, change := range analysis.ChangedFlows {
			fmt.Printf("\n  %s:%d → %s:%d (%s)\n",
				change.Flow.SrcIP, change.Flow.SrcPort,
				change.Flow.DstIP, change.Flow.DstPort,
				change.Flow.Protocol)
			fmt.Printf("    %s → %s\n", change.PreviousAction, change.NewAction)
		}
	}

	// Generate full report if requested
	if options.GenerateReport {
		report := integratedEngine.GenerateTrafficReport(analysis)
		reportPath := "traffic-impact-report.md"
		if options.ReportPath != "" {
			reportPath = options.ReportPath
		}

		err = os.WriteFile(reportPath, []byte(report), 0644)
		if err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("\n📄 Detailed report saved to: %s\n", reportPath)
	}

	return nil
}

// RunConfigTrafficMonitor starts real-time traffic monitoring
func RunConfigTrafficMonitor(currentConfigPath, proposedConfigPath string, options MonitorOptions) error {
	// Load configurations
	currentCfg, err := config.LoadFile(currentConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load current config: %w", err)
	}

	proposedCfg, err := config.LoadFile(proposedConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load proposed config: %w", err)
	}

	integratedEngine := engine.NewIntegratedEngine()
	monitor, err := integratedEngine.StartRealTimeMonitoring(currentCfg, proposedCfg)
	if err != nil {
		return fmt.Errorf("failed to start monitoring: %w", err)
	}
	defer monitor.Stop()

	fmt.Printf("🔍 Real-time Traffic Impact Monitoring\n")
	fmt.Printf("=====================================\n\n")
	fmt.Printf("Monitoring traffic impacts...\n")
	fmt.Printf("Press Ctrl+C to stop\n\n")

	// Listen for impacts
	impactChan := monitor.GetImpactChannel()
	for {
		select {
		case impact := <-impactChan:
			fmt.Printf("[%s] %s:%d → %s:%d (%s): %s → %s\n",
				time.Now().Format("15:04:05"),
				impact.Flow.SrcIP, impact.Flow.SrcPort,
				impact.Flow.DstIP, impact.Flow.DstPort,
				impact.Flow.Protocol,
				impact.PreviousAction, impact.NewAction)
		case <-time.After(10 * time.Second):
			fmt.Printf(".")
		}
	}
}

// TrafficImpactOptions contains options for traffic impact analysis
type TrafficImpactOptions struct {
	WindowMinutes  int
	Verbose        bool
	GenerateReport bool
	ReportPath     string
}

// MonitorOptions contains options for real-time monitoring
type MonitorOptions struct {
	Verbose bool
}
