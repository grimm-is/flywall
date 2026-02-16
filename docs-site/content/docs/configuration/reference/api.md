---
title: "api"
linkTitle: "api"
weight: 21
description: >
  API configuration
---

API configuration

## Syntax

```hcl
api {
  enabled = true
  disable_sandbox = true
  listen = "..."
  tls_listen = "..."
  tls_cert = "..."
  # ...
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `disable_sandbox` | `bool` | No | Default: false (Sandbox Enabled) |
| `listen` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.listen |
| `tls_listen` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.tls_listen |
| `tls_cert` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.tls_cert |
| `tls_key` | `string` | No | ⚠️ *Deprecated.* Deprecated: use web.tls_key |
| `disable_http_redirect` | `bool` | No | ⚠️ *Deprecated.* Deprecated: use web.disable_redirect |
| `require_auth` | `bool` | No | Require API key auth |
| `bootstrap_key` | `string` | No | Bootstrap key (for initial setup, should be removed after creating real keys) |
| `key_store_path` | `string` | No | API key storage Path to key store file |
| `cors_origins` | `list(string)` | No | CORS settings |

## Nested Blocks

### key

Predefined API keys (for config-based key management)

```hcl
key "name" {
  key = "..."
  permissions = [...]
  allowed_ips = [...]
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | `string` | Yes | The actual key value |
| `permissions` | `list(string)` | Yes | Permission strings |
| `allowed_ips` | `list(string)` | No |  |
| `allowed_paths` | `list(string)` | No |  |
| `rate_limit` | `number` | No |  |
| `enabled` | `bool` | No |  |
| `description` | `string` | No |  |

### letsencrypt

Let's Encrypt automatic TLS

```hcl
letsencrypt {
  enabled = true
  email = "..."
  domain = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |
| `email` | `string` | Yes | Contact email for certificate |
| `domain` | `string` | Yes | Domain name for certificate |
| `cache_dir` | `string` | No | Certificate cache directory |
| `staging` | `bool` | No | Use staging server for testing |
