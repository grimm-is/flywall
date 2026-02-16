---
title: "features"
linkTitle: "features"
weight: 28
description: >
  Feature Flags
---

Feature Flags

## Syntax

```hcl
features {
  threat_intel = true
  network_learning = true
  qos = true
  integrity_monitoring = true
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `threat_intel` | `bool` | No | Phase 5: Threat Intelligence |
| `network_learning` | `bool` | No | Automated rule learning |
| `qos` | `bool` | No | Traffic Shaping |
| `integrity_monitoring` | `bool` | No | Detect and revert external changes |
