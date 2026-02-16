---
title: "ntp"
linkTitle: "ntp"
weight: 38
description: >
  NTP configuration
---

NTP configuration

## Syntax

```hcl
ntp {
  enabled = true
  servers = [...]
  interval = "..."
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `servers` | `list(string)` | No | Upstream servers |
| `interval` | `string` | No | Sync interval (e.g. "4h") |
