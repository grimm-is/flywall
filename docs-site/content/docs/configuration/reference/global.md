---
title: "Global Settings"
linkTitle: "Global"
weight: 5
description: >
  Top-level configuration attributes.
---

These attributes are set at the top level of your configuration file.

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `schema_version` | `string` | No (default: `"1.0"`) | Schema version for backward compatibility. Values: `1.0` |
| `ip_forwarding` | `bool` | No (default: `false`) | Enable IP forwarding between interfaces (required for routing). |
| `mss_clamping` | `bool` | No (default: `false`) | Enable TCP MSS clamping to PMTU (recommended for VPNs). |
| `enable_flow_offload` | `bool` | No (default: `false`) | Enable hardware flow offloading for improved performance. |
| `state_dir` | `string` | No | State Directory (overrides default /var/lib/flywall) |
| `log_dir` | `string` | No | Log Directory (overrides default /var/log/flywall) |

## Example

```hcl
schema_version = "1.0"
ip_forwarding  = true
ipv6_forwarding = false
state_dir      = "/var/lib/flywall"
log_dir        = "/var/log/flywall"
```
