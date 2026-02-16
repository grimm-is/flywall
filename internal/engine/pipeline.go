// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package engine

import (
	"context"
	"fmt"
	"time"

	"grimm.is/flywall/internal/config"
)

// ConfigPipeline manages staged configuration validation and application
type ConfigPipeline struct {
	stages []ConfigStage
	engine *IntegratedEngine
}

// ConfigStage represents a single stage in the configuration pipeline
type ConfigStage struct {
	Name        string
	Description string
	Validator   func(*config.Config) error
	Transformer func(*config.Config) (*config.Config, error)
	Executor    func(*config.Config) error
	Optional    bool // If true, pipeline continues on failure
}

// PipelineResult contains the results of pipeline execution
type PipelineResult struct {
	StageResults   map[string]*StageResult
	OverallSuccess bool
	Duration       time.Duration
	Timestamp      time.Time
	TotalErrors    int
	TotalWarnings  int
}

// StageResult contains the result of a single pipeline stage
type StageResult struct {
	Success  bool
	Error    error
	Warnings []string
	Duration time.Duration
	Output   interface{}
	Metrics  map[string]interface{}
}

// NewConfigPipeline creates a new configuration pipeline
func NewConfigPipeline(engine *IntegratedEngine) *ConfigPipeline {
	return &ConfigPipeline{
		engine: engine,
		stages: []ConfigStage{
			{
				Name:        "syntax-validation",
				Description: "Validate configuration syntax and structure",
				Validator:   validateSyntax,
			},
			{
				Name:        "intent-validation",
				Description: "Validate configuration intent and detect logical conflicts",
				Validator:   validateIntent(engine),
			},
			{
				Name:        "deep-validation",
				Description: "Perform comprehensive cross-reference validation",
				Validator:   validateDeep,
			},
			{
				Name:        "compliance-check",
				Description: "Check compliance against security policies",
				Validator:   checkCompliance(engine),
				Optional:    true,
			},
			{
				Name:        "dependency-analysis",
				Description: "Analyze configuration dependencies",
				Validator:   analyzeDependencies(engine),
			},
			{
				Name:        "simulation",
				Description: "Simulate network connectivity scenarios",
				Executor:    simulateConfig(engine),
				Optional:    true,
			},
			{
				Name:        "optimization",
				Description: "Optimize firewall rules",
				Transformer: optimizeConfig,
			},
			{
				Name:        "dry-run",
				Description: "Generate and preview firewall script",
				Executor:    dryRunConfig(engine),
			},
		},
	}
}

// Execute runs the configuration pipeline
func (cp *ConfigPipeline) Execute(ctx context.Context, cfg *config.Config) (*PipelineResult, error) {
	result := &PipelineResult{
		StageResults: make(map[string]*StageResult),
		Timestamp:    time.Now(),
	}

	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	for _, stage := range cp.stages {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			result.OverallSuccess = false
			return result, fmt.Errorf("pipeline cancelled: %w", ctx.Err())
		default:
		}

		stageStart := time.Now()
		stageResult := &StageResult{
			Metrics: make(map[string]interface{}),
		}

		// Execute validator if present
		if stage.Validator != nil {
			if err := stage.Validator(cfg); err != nil {
				stageResult.Success = false
				stageResult.Error = err
				result.TotalErrors++
			} else {
				stageResult.Success = true
			}
		}

		// Execute transformer if present
		if stage.Transformer != nil && (stageResult.Success || stage.Optional) {
			transformed, err := stage.Transformer(cfg)
			if err != nil {
				stageResult.Success = false
				stageResult.Error = err
				result.TotalErrors++
			} else {
				stageResult.Success = true
				stageResult.Output = transformed
				// Use transformed config for subsequent stages
				cfg = transformed
			}
		}

		// Execute executor if present
		if stage.Executor != nil && (stageResult.Success || stage.Optional) {
			if err := stage.Executor(cfg); err != nil {
				stageResult.Success = false
				stageResult.Error = err
				result.TotalErrors++
			} else {
				stageResult.Success = true
			}
		}

		stageResult.Duration = time.Since(stageStart)
		result.StageResults[stage.Name] = stageResult

		// Count warnings
		if stageResult.Error != nil && stage.Optional {
			result.TotalWarnings++
		}

		// Fail pipeline if critical stage failed
		if !stageResult.Success && !stage.Optional {
			result.OverallSuccess = false
			return result, fmt.Errorf("pipeline failed at stage: %s - %v", stage.Name, stageResult.Error)
		}
	}

	result.OverallSuccess = true
	return result, nil
}

// ExecuteWithTimeout runs the pipeline with a timeout
func (cp *ConfigPipeline) ExecuteWithTimeout(cfg *config.Config, timeout time.Duration) (*PipelineResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return cp.Execute(ctx, cfg)
}

// AddStage adds a custom stage to the pipeline
func (cp *ConfigPipeline) AddStage(stage ConfigStage) {
	cp.stages = append(cp.stages, stage)
}

// RemoveStage removes a stage from the pipeline
func (cp *ConfigPipeline) RemoveStage(name string) {
	for i, stage := range cp.stages {
		if stage.Name == name {
			cp.stages = append(cp.stages[:i], cp.stages[i+1:]...)
			break
		}
	}
}

// GetStages returns all pipeline stages
func (cp *ConfigPipeline) GetStages() []ConfigStage {
	return cp.stages
}

// validateSyntax validates configuration syntax
func validateSyntax(cfg *config.Config) error {
	errors := cfg.Validate()
	if len(errors) > 0 {
		return fmt.Errorf("syntax validation failed: %v", errors)
	}
	return nil
}

// validateIntent creates an intent validation function
func validateIntent(engine *IntegratedEngine) func(*config.Config) error {
	return func(cfg *config.Config) error {
		engine.configValidator = config.NewIntentValidator(cfg)
		intentErrors := engine.configValidator.ValidateIntent()
		if len(intentErrors) > 0 {
			return fmt.Errorf("intent validation failed with %d errors", len(intentErrors))
		}
		return nil
	}
}

// validateDeep performs deep validation
func validateDeep(cfg *config.Config) error {
	errors := cfg.DeepValidate()
	if len(errors) > 0 {
		return fmt.Errorf("deep validation failed with %d errors", len(errors))
	}
	return nil
}

// checkCompliance creates a compliance checking function.
// Stub: always passes until compliance feature is implemented.
func checkCompliance(engine *IntegratedEngine) func(*config.Config) error {
	return func(cfg *config.Config) error {
		return nil
	}
}

// analyzeDependencies creates a dependency analysis function.
// Stub: always passes until dependency analysis feature is implemented.
func analyzeDependencies(engine *IntegratedEngine) func(*config.Config) error {
	return func(cfg *config.Config) error {
		return nil
	}
}

// simulateConfig creates a simulation function
func simulateConfig(engine *IntegratedEngine) func(*config.Config) error {
	return func(cfg *config.Config) error {
		results := engine.simulateCriticalScenarios(cfg)

		failedCount := 0
		for _, result := range results {
			if !result.Passed && !result.Warning {
				failedCount++
			}
		}

		if failedCount > 0 {
			return fmt.Errorf("simulation failed for %d scenarios", failedCount)
		}
		return nil
	}
}

// optimizeConfig optimizes the configuration.
// Stub: returns config unmodified until optimization feature is implemented.
func optimizeConfig(cfg *config.Config) (*config.Config, error) {
	return cfg, nil
}

// dryRunConfig performs a dry run of the configuration
func dryRunConfig(engine *IntegratedEngine) func(*config.Config) error {
	return func(cfg *config.Config) error {
		_, err := engine.DryRun(cfg)
		if err != nil {
			return fmt.Errorf("dry run failed: %w", err)
		}
		return nil
	}
}

// PrintSummary prints a human-readable summary of pipeline results
func (pr *PipelineResult) PrintSummary() {
	fmt.Printf("\nPipeline Execution Results (%.2fs):\n", pr.Duration.Seconds())
	fmt.Printf("Overall Success: %t\n", pr.OverallSuccess)
	fmt.Printf("Total Errors: %d\n", pr.TotalErrors)
	fmt.Printf("Total Warnings: %d\n", pr.TotalWarnings)
	fmt.Printf("\nStage Results:\n")
	for name, result := range pr.StageResults {
		status := "✅"
		if !result.Success {
			status = "⚠️"
		}
		fmt.Printf("  %s %s: %.2fs", status, name, result.Duration.Seconds())
		if result.Error != nil {
			fmt.Printf(" - %s", result.Error.Error())
		}
		fmt.Printf("\n")
	}
}
