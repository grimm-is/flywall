---
title: "scheduler"
linkTitle: "scheduler"
weight: 48
description: >
  SchedulerConfig defines scheduler settings.
---

SchedulerConfig defines scheduler settings.

## Syntax

```hcl
scheduler {
  enabled = true
  ipset_refresh_hours = 0
  dns_refresh_hours = 0
  backup_enabled = true
  backup_schedule = "..."
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `ipset_refresh_hours` | `number` | No | Default: 24 |
| `dns_refresh_hours` | `number` | No | Default: 24 |
| `backup_enabled` | `bool` | No | Enable auto backups |
| `backup_schedule` | `string` | No | Cron expression, default: "0 2 * * *" |
| `backup_retention_days` | `number` | No | Default: 7 |
| `backup_dir` | `string` | No | Default: /var/lib/firewall/backups |
