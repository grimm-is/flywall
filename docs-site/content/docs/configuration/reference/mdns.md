---
title: "mdns"
linkTitle: "mdns"
weight: 34
description: >
  mDNS Reflector configuration
---

mDNS Reflector configuration

## Syntax

```hcl
mdns {
  enabled = true
  interfaces = [...]
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `interfaces` | `list(string)` | No | Interfaces to reflect between |
