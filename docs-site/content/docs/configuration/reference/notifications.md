---
title: "notifications"
linkTitle: "notifications"
weight: 37
description: >
  NotificationsConfig configures the notification system.
---

NotificationsConfig configures the notification system.

## Syntax

```hcl
notifications {
  enabled = true

  channel { ... }

  rule { ... }
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |

## Nested Blocks

### channel

NotificationChannel defines a notification destination.

```hcl
channel "name" {
  type = "..."
  level = "..."
  enabled = true
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | `string` | Yes | email, pushover, slack, discord, ntfy, webhook |
| `level` | `string` | No | critical, warning, info |
| `enabled` | `bool` | No |  |
| `smtp_host` | `string` | No | Email settings |
| `smtp_port` | `number` | No |  |
| `smtp_user` | `string` | No |  |
| `smtp_password` | `string` | No |  |
| `from` | `string` | No |  |
| `to` | `list(string)` | No |  |
| `webhook_url` | `string` | No | Webhook/Slack/Discord settings |
| `channel` | `string` | No |  |
| `username` | `string` | No |  |
| `api_token` | `string` | No | Pushover settings |
| `user_key` | `string` | No |  |
| `priority` | `number` | No |  |
| `sound` | `string` | No |  |
| `server` | `string` | No | ntfy settings |
| `topic` | `string` | No |  |
| `password` | `string` | No | Generic auth (for ntfy, webhook) |
| `headers` | `map` | No |  |

### rule

AlertRule defines when an alert should be triggered.

```hcl
rule "name" {
  enabled = true
  condition = "..."
  severity = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `condition` | `string` | Yes |  |
| `severity` | `string` | No | info, warning, critical |
| `channels` | `list(string)` | No |  |
| `cooldown` | `string` | No | e.g. "1h" |
