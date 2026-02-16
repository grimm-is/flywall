// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package coordinator

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// CoordinatorConfig represents configuration for the coordinator
type CoordinatorConfig struct {
	Adaptive           bool
	ScaleBackThreshold float64
	ScaleBackRate      float64
	MinimumFeatures    []string
	SamplingConfig     SamplingConfig
	Performance        PerformanceConfig
}

// PerformanceConfig represents performance configuration
type PerformanceConfig struct {
	MaxCPUPercent   float64
	MaxMemoryMB     int
	MaxEventsPerSec int
	MaxPPS          float64
}

// SamplingConfig represents sampling configuration
type SamplingConfig struct {
	Enabled       bool
	MinSampleRate float64
	MaxSampleRate float64
	AdaptiveRate  bool
}

// Coordinator manages eBPF feature dependencies and priorities
type Coordinator struct {
	features     map[string]*Feature
	dependencies map[string][]string // feature -> dependencies
	dependents   map[string][]string // feature -> dependent features
	mutex        sync.RWMutex
	config       *CoordinatorConfig
	state        *CoordinatorState
}

// Feature represents an eBPF feature with its state
type Feature struct {
	Name         string
	Enabled      bool
	Active       bool
	Priority     int
	Dependencies []string
	Cost         ResourceCost
	Status       FeatureStatus
	LastUpdated  time.Time
	mutex        sync.RWMutex
}

// FeatureStatus represents the status of a feature
type FeatureStatus struct {
	Active       bool
	LoadedAt     time.Time
	LastActive   time.Time
	Error        string
	SamplingRate float64
}

// ResourceCost represents resource usage
type ResourceCost struct {
	CPU          float64
	Memory       int
	MapLookups   float64
	EventsPerSec int
	MaxPPS       float64
}

// CoordinatorState tracks the overall coordinator state
type CoordinatorState struct {
	TotalCPU       float64   `json:"total_cpu"`
	TotalMemory    int       `json:"total_memory"`
	ActiveFeatures int       `json:"active_features"`
	LastScaleBack  time.Time `json:"last_scale_back"`
	AdaptiveMode   bool      `json:"adaptive_mode"`
}

// NewCoordinator creates a new feature coordinator
func NewCoordinator(config *CoordinatorConfig) *Coordinator {
	return &Coordinator{
		features:     make(map[string]*Feature),
		dependencies: make(map[string][]string),
		dependents:   make(map[string][]string),
		config:       config,
		state: &CoordinatorState{
			AdaptiveMode: config.Adaptive,
		},
	}
}

// RegisterFeature registers a feature with the coordinator
func (c *Coordinator) RegisterFeature(feature *Feature) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.features[feature.Name]; exists {
		return fmt.Errorf("feature %s already registered", feature.Name)
	}

	// Create feature
	f := &Feature{
		Name:         feature.Name,
		Enabled:      feature.Enabled,
		Priority:     feature.Priority,
		Dependencies: feature.Dependencies,
		Cost:         feature.Cost,
		Status:       feature.Status,
		LastUpdated:  time.Now(),
	}

	c.features[feature.Name] = f
	c.dependencies[feature.Name] = feature.Dependencies

	// Update dependents mapping
	for _, dep := range feature.Dependencies {
		if c.dependents[dep] == nil {
			c.dependents[dep] = make([]string, 0)
		}
		c.dependents[dep] = append(c.dependents[dep], feature.Name)
	}

	return nil
}

// EnableFeature enables a feature if dependencies allow
func (c *Coordinator) EnableFeature(name string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	feature, exists := c.features[name]
	if !exists {
		return fmt.Errorf("feature %s not found", name)
	}

	// Check dependencies
	if err := c.checkDependencies(name); err != nil {
		return fmt.Errorf("cannot enable %s: %w", name, err)
	}

	// Check resource constraints
	if err := c.checkResourceConstraints(feature, true); err != nil {
		return fmt.Errorf("cannot enable %s: %w", name, err)
	}

	feature.mutex.Lock()
	feature.Enabled = true
	feature.Active = true
	feature.Status.Active = true
	feature.Status.LoadedAt = time.Now()
	feature.LastUpdated = time.Now()
	feature.mutex.Unlock()

	// Update state
	c.updateResourceUsage()

	return nil
}

// DisableFeature disables a feature and its dependents
func (c *Coordinator) DisableFeature(name string, force bool) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	feature, exists := c.features[name]
	if !exists {
		return fmt.Errorf("feature %s not found", name)
	}

	// Check dependents unless forced
	if !force {
		if dependents := c.dependents[name]; len(dependents) > 0 {
			return fmt.Errorf("cannot disable %s: has active dependents %v", name, dependents)
		}
	}

	// Disable feature
	feature.mutex.Lock()
	feature.Enabled = false
	feature.Active = false
	feature.Status.Active = false
	feature.LastUpdated = time.Now()
	feature.mutex.Unlock()

	// Force disable dependents if forced
	if force {
		for _, dependent := range c.dependents[name] {
			if depFeature := c.features[dependent]; depFeature != nil && depFeature.Active {
				c.disableFeatureInternal(dependent)
			}
		}
	}

	// Update state
	c.updateResourceUsage()

	return nil
}

// disableFeatureInternal disables a feature (internal, assumes lock held)
func (c *Coordinator) disableFeatureInternal(name string) {
	if feature := c.features[name]; feature != nil {
		feature.mutex.Lock()
		feature.Enabled = false
		feature.Active = false
		feature.Status.Active = false
		feature.LastUpdated = time.Now()
		feature.mutex.Unlock()
	}
}

// checkDependencies checks if all dependencies are satisfied
func (c *Coordinator) checkDependencies(name string) error {
	deps := c.dependencies[name]

	for _, dep := range deps {
		depFeature, exists := c.features[dep]
		if !exists {
			return fmt.Errorf("dependency %s not found", dep)
		}

		if !depFeature.Active {
			return fmt.Errorf("dependency %s is not active", dep)
		}
	}

	return nil
}

// checkResourceConstraints checks if enabling a feature would exceed resource limits
func (c *Coordinator) checkResourceConstraints(feature *Feature, enabling bool) error {
	if !c.config.Adaptive {
		return nil
	}

	// Calculate new totals
	newCPU := c.state.TotalCPU
	newMemory := c.state.TotalMemory

	if enabling {
		newCPU += feature.Cost.CPU
		newMemory += feature.Cost.Memory
	} else {
		newCPU -= feature.Cost.CPU
		newMemory -= feature.Cost.Memory
	}

	// Check CPU limit
	if newCPU > c.config.Performance.MaxCPUPercent {
		return fmt.Errorf("enabling feature would exceed CPU limit: %.2f > %.2f",
			newCPU, c.config.Performance.MaxCPUPercent)
	}

	// Check memory limit
	if newMemory > c.config.Performance.MaxMemoryMB {
		return fmt.Errorf("enabling feature would exceed memory limit: %d > %d",
			newMemory, c.config.Performance.MaxMemoryMB)
	}

	return nil
}

// updateResourceUsage updates the current resource usage
func (c *Coordinator) updateResourceUsage() {
	totalCPU := 0.0
	totalMemory := 0
	activeCount := 0

	for _, feature := range c.features {
		if feature.Active {
			totalCPU += feature.Cost.CPU
			totalMemory += feature.Cost.Memory
			activeCount++
		}
	}

	c.state.TotalCPU = totalCPU
	c.state.TotalMemory = totalMemory
	c.state.ActiveFeatures = activeCount
}

// ScaleBack scales back features based on resource pressure
func (c *Coordinator) ScaleBack() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.config.Adaptive {
		return nil
	}

	// Check if we need to scale back
	if c.state.TotalCPU <= c.config.ScaleBackThreshold {
		return nil
	}

	// Get features to disable (lowest priority first)
	toDisable := c.getFeaturesForScaleBack()

	// Disable features until we're under the threshold
	for _, name := range toDisable {
		feature := c.features[name]
		if !feature.Active {
			continue
		}

		// Don't disable minimum features
		if c.isMinimumFeature(name) {
			continue
		}

		// Disable the feature
		c.disableFeatureInternal(name)

		// Adjust sampling rates
		for _, feature := range c.features {
			if !feature.Active {
				continue
			}

			// Reduce sampling rate for less critical features
			if feature.Status.SamplingRate > c.config.SamplingConfig.MinSampleRate {
				feature.Status.SamplingRate *= c.config.ScaleBackRate
				if feature.Status.SamplingRate < c.config.SamplingConfig.MinSampleRate {
					feature.Status.SamplingRate = c.config.SamplingConfig.MinSampleRate
				}
			}
		}

		// Update state
		c.updateResourceUsage()
		c.state.LastScaleBack = time.Now()

		// Check if we're under the threshold now
		if c.state.TotalCPU <= c.config.ScaleBackThreshold {
			break
		}
	}

	return nil
}

// getFeaturesForScaleBack returns features ordered by priority (lowest first)
func (c *Coordinator) getFeaturesForScaleBack() []string {
	features := make([]*Feature, 0, len(c.features))

	for _, feature := range c.features {
		if feature.Active {
			features = append(features, feature)
		}
	}

	// Sort by priority (lowest first)
	sort.Slice(features, func(i, j int) bool {
		return features[i].Priority < features[j].Priority
	})

	names := make([]string, len(features))
	for i, feature := range features {
		names[i] = feature.Name
	}

	return names
}

// isMinimumFeature checks if a feature is in the minimum features list
func (c *Coordinator) isMinimumFeature(name string) bool {
	for _, min := range c.config.MinimumFeatures {
		if min == name {
			return true
		}
	}
	return false
}

// ScaleUp scales up features when resources are available
func (c *Coordinator) ScaleUp() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.config.Adaptive {
		return nil
	}

	// Check if we can scale up
	if c.state.TotalCPU > c.config.ScaleBackThreshold*0.8 {
		return nil
	}

	// Get features to enable (highest priority first)
	toEnable := c.getFeaturesForScaleUp()

	// Enable features if we have resources
	for _, name := range toEnable {
		feature := c.features[name]
		if feature.Active {
			continue
		}

		// Check if we can enable it
		if err := c.checkDependencies(name); err != nil {
			continue
		}

		if err := c.checkResourceConstraints(feature, true); err != nil {
			continue
		}

		// Enable the feature
		feature.mutex.Lock()
		feature.Enabled = true
		feature.Active = true
		feature.Status.Active = true
		feature.Status.LoadedAt = time.Now()
		feature.LastUpdated = time.Now()
		feature.mutex.Unlock()

		// Update state
		c.updateResourceUsage()
	}

	return nil
}

// getFeaturesForScaleUp returns disabled features ordered by priority (highest first)
func (c *Coordinator) getFeaturesForScaleUp() []string {
	features := make([]*Feature, 0, len(c.features))

	for _, feature := range c.features {
		if !feature.Active && feature.Enabled {
			features = append(features, feature)
		}
	}

	// Sort by priority (highest first)
	sort.Slice(features, func(i, j int) bool {
		return features[i].Priority > features[j].Priority
	})

	names := make([]string, len(features))
	for i, feature := range features {
		names[i] = feature.Name
	}

	return names
}

// GetFeatureStatus returns the status of all features
func (c *Coordinator) GetFeatureStatus() map[string]FeatureStatus {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	status := make(map[string]FeatureStatus)

	for name, feature := range c.features {
		feature.mutex.RLock()
		status[name] = feature.Status
		feature.mutex.RUnlock()
	}

	return status
}

// GetCoordinatorState returns the coordinator state
func (c *Coordinator) GetCoordinatorState() *CoordinatorState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Return a copy to avoid race conditions
	return &CoordinatorState{
		TotalCPU:       c.state.TotalCPU,
		TotalMemory:    c.state.TotalMemory,
		ActiveFeatures: c.state.ActiveFeatures,
		LastScaleBack:  c.state.LastScaleBack,
		AdaptiveMode:   c.state.AdaptiveMode,
	}
}

// ValidateConfiguration validates the feature configuration
func (c *Coordinator) ValidateConfiguration() error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Check for circular dependencies
	if err := c.checkCircularDependencies(); err != nil {
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	// Check if all dependencies exist
	for name, deps := range c.dependencies {
		for _, dep := range deps {
			if _, exists := c.features[dep]; !exists {
				return fmt.Errorf("feature %s depends on non-existent feature %s", name, dep)
			}
		}
	}

	// Check priority conflicts
	priorities := make(map[int][]string)
	for name, feature := range c.features {
		priorities[feature.Priority] = append(priorities[feature.Priority], name)
	}

	// Warn about same priorities (but don't fail)
	for priority, features := range priorities {
		if len(features) > 1 {
			// Log warning
			fmt.Printf("Warning: Features %v have same priority %d\n", features, priority)
		}
	}

	return nil
}

// checkCircularDependencies checks for circular dependencies
func (c *Coordinator) checkCircularDependencies() error {
	visiting := make(map[string]bool)
	visited := make(map[string]bool)

	for name := range c.features {
		if !visited[name] {
			if err := c.visitNode(name, visiting, visited); err != nil {
				return err
			}
		}
	}

	return nil
}

// visitNode visits a node in the dependency graph
func (c *Coordinator) visitNode(name string, visiting, visited map[string]bool) error {
	if visiting[name] {
		return fmt.Errorf("circular dependency involving %s", name)
	}

	if visited[name] {
		return nil
	}

	visiting[name] = true
	defer delete(visiting, name)

	for _, dep := range c.dependencies[name] {
		if err := c.visitNode(dep, visiting, visited); err != nil {
			return err
		}
	}

	visited[name] = true
	return nil
}

// UpdateFeatureCost updates the resource cost of a feature
func (c *Coordinator) UpdateFeatureCost(name string, cost ResourceCost) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	feature, exists := c.features[name]
	if !exists {
		return fmt.Errorf("feature %s not found", name)
	}

	feature.mutex.Lock()
	oldCost := feature.Cost
	feature.Cost = cost
	feature.LastUpdated = time.Now()
	feature.mutex.Unlock()

	// Update totals
	if feature.Active {
		c.state.TotalCPU += cost.CPU - oldCost.CPU
		c.state.TotalMemory += cost.Memory - oldCost.Memory
	}

	return nil
}

// GetDependencyGraph returns the dependency graph
func (c *Coordinator) GetDependencyGraph() map[string][]string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	graph := make(map[string][]string)
	for name, deps := range c.dependencies {
		graph[name] = make([]string, len(deps))
		copy(graph[name], deps)
	}

	return graph
}

// Close closes the coordinator
func (c *Coordinator) Close() error {
	// Disable all features
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for name := range c.features {
		c.disableFeatureInternal(name)
	}

	return nil
}
