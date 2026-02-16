---
title: "cloud"
linkTitle: "cloud"
weight: 23
description: >
  Cloud Management
---

Cloud Management

## Syntax

```hcl
cloud {
  enabled = true
  hub_address = "..."
  device_id = "..."
  cert_file = "..."
  key_file = "..."
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `hub_address` | `string` | No |  |
| `device_id` | `string` | No |  |
| `cert_file` | `string` | No | Certificate paths (override defaults) |
| `key_file` | `string` | No |  |
| `ca_file` | `string` | No |  |
| `local_priority` | `list(string)` | No | Local config priority field |
