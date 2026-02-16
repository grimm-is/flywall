---
title: "zone"
linkTitle: "zone"
weight: 57
description: >
  Zone defines a network security zone. Zones can match traffic by interface, source/destination IP...
---

Zone defines a network security zone.
Zones can match traffic by interface, source/destination IP, VLAN, or combinations.
Simple zones use top-level fields, complex zones use match blocks.

## Syntax

```hcl
zone "name" {
  color = "..."
  description = "..."
  interface = "..."
  src = "..."
  dst = "..."
  # ...
}
```

## Labels

| Label | Description | Required |
|-------|-------------|----------|
| `name` |  | Yes |

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `color` | `string` | No |  |
| `description` | `string` | No |  |
| `interface` | `string` | No | Simple match criteria (use for single-interface zones) These are effectively ... |
| `src` | `string` | No | Source IP/network (e.g., "192.168.1.0/24") |
| `dst` | `string` | No | Destination IP/network |
| `vlan` | `number` | No | VLAN tag |
| `interfaces` | `list(string)` | No | ⚠️ *Deprecated.* DEPRECATED: Use Interface or Matches instead Will be auto-converted to Matche... |
| `ipsets` | `list(string)` | No | Legacy fields (kept for backwards compat) IPSet names for IP-based membership |
| `networks` | `list(string)` | No | Direct CIDR ranges |
| `action` | `string` | No | Zone behavior Action for intra-zone traffic: "accept", "drop", "reject" (defa... Values: `accept`, `reject` |
| `external` | `bool` | No | External marks this as an external/WAN zone (used for auto-masquerade detecti... |
| `ipv4` | `list(string)` | No | IP assignment for simple zones (shorthand - assigns to the interface) |
| `ipv6` | `list(string)` | No |  |
| `dhcp` | `bool` | No | Use DHCP client on this interface |

## Nested Blocks

### match

Complex match criteria (OR logic between matches, AND logic within each match)
Global fields above apply to ALL matches as defaults

```hcl
match {
  interface = "..."
  src = "..."
  dst = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `interface` | `string` | No | Interface can be exact ("eth0") or prefix with + or * suffix ("wg+" matches w... |
| `src` | `string` | No |  |
| `dst` | `string` | No |  |
| `vlan` | `number` | No |  |
| `protocol` | `string` | No | Advanced matching |
| `mac` | `string` | No |  |
| `dscp` | `string` | No | value, class, or classid |
| `mark` | `string` | No |  |
| `tos` | `number` | No |  |
| `out_interface` | `string` | No |  |
| `phys_in` | `string` | No |  |
| `phys_out` | `string` | No |  |

### services

Services provided TO this zone (firewall auto-generates rules)
These define what the firewall offers to clients in this zone

```hcl
services {
  dhcp = true
  dns = true
  ntp = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `dhcp` | `bool` | No | Network services Allow DHCP requests (udp/67-68) |
| `dns` | `bool` | No | Allow DNS queries (udp/53, tcp/53) |
| `ntp` | `bool` | No | Allow NTP sync (udp/123) |
| `captive_portal` | `bool` | No | Captive portal / guest access Redirect HTTP to portal |

#### port

Custom service ports (auto-allow)

```hcl
port "name" {
  protocol = "..."
  port = 0
  port_end = 0
}
```

**Labels:**

- `name` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `protocol` | `string` | Yes | tcp, udp |
| `port` | `number` | Yes | Port number |
| `port_end` | `number` | No | For port ranges |

### management

Management access FROM this zone to the firewall

```hcl
management {
  web_ui = true
  web = true
  ssh = true
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `web_ui` | `bool` | No | Legacy: Allow Web UI access (tcp/80, tcp/443) -> Use Web |
| `web` | `bool` | No | Allow Web UI access (tcp/80, tcp/443) |
| `ssh` | `bool` | No | Allow SSH access (tcp/22) |
| `api` | `bool` | No | Allow API access (used for L7 filtering, implies HTTPS access) |
| `icmp` | `bool` | No | Allow ping to firewall |
| `snmp` | `bool` | No | Allow SNMP queries (udp/161) |
| `syslog` | `bool` | No | Allow syslog sending (udp/514) |
