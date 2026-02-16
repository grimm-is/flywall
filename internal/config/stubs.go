// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// ConnectivityTestResult is a stub for connectivity testing results.
type ConnectivityTestResult struct {
	Source     string
	Dest       string
	Protocol   string
	Port       int
	CanConnect bool
	Error      error
	Path       []string
}

// ComplianceReport is a stub for compliance checking results.
type ComplianceReport struct {
	OverallPassed bool
	Score         float64
	Results       []ComplianceResult
	CriticalCount int
	HighCount     int
	MediumCount   int
	LowCount      int
	InfoCount     int
}

// ComplianceResult is a stub for a single compliance check result.
type ComplianceResult struct {
	RuleID      string
	Passed      bool
	Message     string
	Description string
}

// GetStandardCompliancePolicies returns a map of standard compliance policies.
func GetStandardCompliancePolicies() map[string]interface{} {
	return make(map[string]interface{})
}

// DependencyGraph is a stub for dependency analysis.
type DependencyGraph struct {
	Nodes  []string
	Edges  map[string][]string
	Cycles [][]string
}
