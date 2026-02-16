---
title: "rule_learning"
linkTitle: "rule_learning"
weight: 46
description: >
  Rule learning and notifications
---

Rule learning and notifications

## Syntax

```hcl
rule_learning {
  enabled = true
  log_group = 0
  rate_limit = "..."
  auto_approve = true
  ignore_networks = [...]
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `log_group` | `number` | No | nflog group (default: 100) |
| `rate_limit` | `string` | No | e.g., "10/minute" |
| `auto_approve` | `bool` | No | Auto-approve learned rules (legacy) |
| `ignore_networks` | `list(string)` | No | Networks to ignore from learning |
| `retention_days` | `number` | No | How long to keep pending rules |
| `cache_size` | `number` | No | Flow cache size (default: 10000) |
| `learning_mode` | `bool` | No | TOFU (Trust On First Use) mode |
| `inline_mode` | `bool` | No | InlineMode uses nfqueue instead of nflog for packet inspection. This holds pa... |
