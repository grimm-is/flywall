---
title: "qos_policy"
linkTitle: "qos_policy"
weight: 42
description: >
  Per-interface settings (first-class)
---

Per-interface settings (first-class)

## Syntax

```hcl
qos_policy "name" {
  interface = "..."
  enabled = true
  direction = "ingress"
  download_mbps = 0
  upload_mbps = 0
}
```

## Labels

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | Yes | Interface to apply QoS |
| `enabled` | `bool` | No |  |
| `direction` | `string` | No | "ingress", "egress", "both" (default: both) Values: `ingress`, `both` |
| `download_mbps` | `number` | No |  |
| `upload_mbps` | `number` | No |  |

## Nested Blocks

### class

QoSClass defines a traffic class for QoS.

```hcl
class "name" {
  priority = 0
  rate = "..."
  ceil = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `priority` | `number` | No | 1-7, lower is higher priority |
| `rate` | `string` | No | Guaranteed rate e.g., "10mbit" or "10%" |
| `ceil` | `string` | No | Maximum rate |
| `burst` | `string` | No | Burst size |
| `queue_type` | `string` | No | "fq_codel", "sfq", "pfifo" (default: fq_codel) Values: `fq_codel`, `pfifo` |

### rule

Traffic classification rules

```hcl
rule "name" {
  class = "..."
  proto = "..."
  src_ip = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `class` | `string` | Yes | Target QoS class |
| `proto` | `string` | No |  |
| `src_ip` | `string` | No |  |
| `dest_ip` | `string` | No |  |
| `src_port` | `number` | No |  |
| `dest_port` | `number` | No |  |
| `services` | `list(string)` | No | Service names |
| `dscp` | `string` | No | Match DSCP value |
| `set_dscp` | `string` | No | Set DSCP on matching traffic |

#### threat_intel

ThreatIntel configures threat intelligence feeds.

```hcl
threat_intel {
  enabled = true
  interval = "..."
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `interval` | `string` | No | e.g. "1h" |

##### source

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
