---
title: "geoip"
linkTitle: "geoip"
weight: 30
description: >
  GeoIP configuration for country-based filtering
---

GeoIP configuration for country-based filtering

## Syntax

```hcl
geoip {
  enabled = true
  database_path = "..."
  auto_update = true
  license_key = "..."
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No | Enabled activates GeoIP matching in firewall rules. |
| `database_path` | `string` | No | DatabasePath is the path to the MMDB file (MaxMind or DB-IP format). Default:... |
| `auto_update` | `bool` | No | AutoUpdate enables automatic database updates (future feature). |
| `license_key` | `string` | No | LicenseKey for premium MaxMind database updates (future feature). Not require... |
