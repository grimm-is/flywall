---
title: "syslog"
linkTitle: "syslog"
weight: 49
description: >
  Syslog remote logging
---

Syslog remote logging

## Syntax

```hcl
syslog {
  enabled = true
  host = "..."
  port = 0
  protocol = "..."
  tag = "..."
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `host` | `string` | Yes | Remote syslog server hostname/IP |
| `port` | `number` | No | Default: 514 |
| `protocol` | `string` | No | udp or tcp (default: udp) |
| `tag` | `string` | No | Syslog tag (default: flywall) |
| `facility` | `number` | No | Syslog facility (default: 1) |
