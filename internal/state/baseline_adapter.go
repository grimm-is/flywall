package state

import "grimm.is/flywall/internal/clock"

// BaselineAdapter implements metrics.BaselinePersister using MetricsBaselineBucket.
// This bridges the metrics package to the state package without creating a circular import.
type BaselineAdapter struct {
	bucket *MetricsBaselineBucket
}

// NewBaselineAdapter creates a new baseline adapter.
func NewBaselineAdapter(bucket *MetricsBaselineBucket) *BaselineAdapter {
	return &BaselineAdapter{bucket: bucket}
}

// SaveInterfaceBaseline saves the baseline for an interface.
func (a *BaselineAdapter) SaveInterfaceBaseline(name string, rxBytes, txBytes uint64) error {
	return a.bucket.SetInterface(&CounterBaseline{
		Name:    name,
		RxBytes: rxBytes,
		TxBytes: txBytes,
		SavedAt: clock.Now(),
	})
}

// LoadInterfaceBaseline loads the baseline for an interface.
func (a *BaselineAdapter) LoadInterfaceBaseline(name string) (rxBytes, txBytes uint64, err error) {
	baseline, err := a.bucket.GetInterface(name)
	if err != nil {
		return 0, 0, err
	}
	return baseline.RxBytes, baseline.TxBytes, nil
}

// SavePolicyBaseline saves the baseline for a policy.
func (a *BaselineAdapter) SavePolicyBaseline(key string, packets, bytes uint64) error {
	return a.bucket.SetPolicy(&CounterBaseline{
		Name:    key,
		Packets: packets,
		Bytes:   bytes,
		SavedAt: clock.Now(),
	})
}

// LoadPolicyBaseline loads the baseline for a policy.
func (a *BaselineAdapter) LoadPolicyBaseline(key string) (packets, bytes uint64, err error) {
	baseline, err := a.bucket.GetPolicy(key)
	if err != nil {
		return 0, 0, err
	}
	return baseline.Packets, baseline.Bytes, nil
}
