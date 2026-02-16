---
title: "anomaly_detection"
linkTitle: "anomaly_detection"
weight: 20
description: >
  AnomalyConfig configures traffic anomaly detection.
---

AnomalyConfig configures traffic anomaly detection.

## Syntax

```hcl
anomaly_detection {
  enabled = true
  baseline_window = "..."
  min_samples = 0
  spike_stddev = 0
  drop_stddev = 0
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `baseline_window` | `string` | No | e.g., "7d" |
| `min_samples` | `number` | No | Min hits before alerting |
| `spike_stddev` | `number` | No | Alert if > N stddev |
| `drop_stddev` | `number` | No | Alert if < N stddev |
| `alert_cooldown` | `string` | No | e.g., "15m" |
| `port_scan_threshold` | `number` | No | Ports hit before alert |
