---
title: "system"
linkTitle: "system"
weight: 50
description: >
  System tuning and settings
---

System tuning and settings

## Syntax

```hcl
system {
  sysctl_profile = "default"
  sysctl = {...}
  timezone = "..."
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `sysctl_profile` | `string` | No | SysctlProfile selects a preset sysctl tuning profile Options: "default", "per... Values: `default`, `security` |
| `sysctl` | `map` | No | Sysctl allows manual override of sysctl parameters Applied after profile tuning |
| `timezone` | `string` | No | Timezone for scheduled rules (e.g. "America/Los_Angeles"). Defaults to "UTC". |
