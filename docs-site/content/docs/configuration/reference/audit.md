---
title: "audit"
linkTitle: "audit"
weight: 22
description: >
  Audit logging configuration
---

Audit logging configuration

## Syntax

```hcl
audit {
  enabled = true
  retention_days = 0
  kernel_audit = true
  database_path = "..."
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates audit logging to SQLite. |
| `retention_days` | `number` | No | RetentionDays is the number of days to retain audit events. Default: 90 days. |
| `kernel_audit` | `bool` | No | KernelAudit enables writing to the Linux kernel audit log (auditd). Useful fo... |
| `database_path` | `string` | No | DatabasePath overrides the default audit database location. Default: /var/lib... |
