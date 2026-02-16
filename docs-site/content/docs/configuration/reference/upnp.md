---
title: "upnp"
linkTitle: "upnp"
weight: 54
description: >
  UPnP IGD configuration
---

UPnP IGD configuration

## Syntax

```hcl
upnp {
  enabled = true
  external_interface = "..."
  internal_interfaces = [...]
  secure_mode = true
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `external_interface` | `string` | No | WAN interface |
| `internal_interfaces` | `list(string)` | No | LAN interfaces |
| `secure_mode` | `bool` | No | Only allow mapping to requesting IP |
