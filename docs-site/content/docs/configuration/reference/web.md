---
title: "web"
linkTitle: "web"
weight: 56
description: >
  Web Server configuration (previously part of API)
---

Web Server configuration (previously part of API)

## Syntax

```hcl
web {
  listen = "..."
  tls_listen = "..."
  tls_cert = "..."
  tls_key = "..."
  disable_redirect = true
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `listen` | `string` | No | Listen addresses HTTP listen address (default :80) |
| `tls_listen` | `string` | No | HTTPS listen address (default :443) |
| `tls_cert` | `string` | No | TLS Configuration Path to TLS certificate |
| `tls_key` | `string` | No | Path to TLS key |
| `disable_redirect` | `bool` | No | Behavior Disable HTTP->HTTPS redirect |
| `serve_ui` | `bool` | No | Serve the dashboard (default true) |
| `serve_api` | `bool` | No | Serve API paths (default true) |

## Nested Blocks

### allow

Access Control

```hcl
allow {
  interface = "..."
  source = "..."
  interfaces = [...]
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Single value fields |
| `source` | `string` | No |  |
| `interfaces` | `list(string)` | No | List value fields (for brevity) |
| `sources` | `list(string)` | No |  |

### deny

AccessRule defines criteria for allowing or denying access.

```hcl
deny {
  interface = "..."
  source = "..."
  interfaces = [...]
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Single value fields |
| `source` | `string` | No |  |
| `interfaces` | `list(string)` | No | List value fields (for brevity) |
| `sources` | `list(string)` | No |  |
