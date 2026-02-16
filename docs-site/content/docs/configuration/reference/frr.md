---
title: "frr"
linkTitle: "frr"
weight: 29
description: >
  FRRConfig holds configuration for Free Range Routing (FRR).
---

FRRConfig holds configuration for Free Range Routing (FRR).

## Syntax

```hcl
frr {
  enabled = true

  ospf { ... }

  bgp { ... }
}
```

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `enabled` | `bool` | No |  |

## Nested Blocks

### ospf

OSPF configuration.

```hcl
ospf {
  router_id = "..."
  networks = [...]
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `router_id` | `string` | No |  |
| `networks` | `list(string)` | No | List of CIDRs to advertise |

#### area

OSPFArea configuration.

```hcl
area "id" {
  networks = [...]
}
```

**Labels:**

- `id` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `networks` | `list(string)` | No |  |

### bgp

BGP configuration.

```hcl
bgp {
  asn = 0
  router_id = "..."
  networks = [...]
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `asn` | `number` | No |  |
| `router_id` | `string` | No |  |
| `networks` | `list(string)` | No |  |

#### neighbor

Neighbor BGP peer configuration.

```hcl
neighbor "ip" {
  remote_asn = 0
}
```

**Labels:**

- `ip` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `remote_asn` | `number` | No |  |
