---
title: "threat_intel"
linkTitle: "threat_intel"
weight: 51
description: >
  ThreatIntel configures threat intelligence feeds.
---

ThreatIntel configures threat intelligence feeds.

## Syntax

```hcl
threat_intel {
  enabled = true
  interval = "..."

  source { ... }
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `interval` | `string` | No | e.g. "1h" |

## Nested Blocks

### source

```hcl
source "name" {
  url = "..."
  format = "taxii"
  collection_id = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | `string` | Yes |  |
| `format` | `string` | No | "taxii", "text", "json" Values: `taxii`, `json` |
| `collection_id` | `string` | No |  |
| `username` | `string` | No |  |
| `password` | `string` | No |  |
