---
title: "interface"
linkTitle: "interface"
weight: 31
description: >
  Interface represents a physical or virtual network interface configuration. Each interface can be...
---

Interface represents a physical or virtual network interface configuration.
Each interface can be assigned to a security zone and configured with
static IPs, DHCP, VLANs, and other network settings.

## Syntax

```hcl
interface "name" {
  description = "WAN Uplink"
  disabled = false
  zone = "wan"
  ipv4 = ["192.168.1.1/24"]
  ipv6 = ["2001:db8::1/64"]
  # ...
}
```

## Labels

| Label | Description | Required |
|-------|-------------|----------|
| `name` | Name is the system interface name. | Yes |

## Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No | Human-readable description for this interface. |
| `disabled` | `bool` | No (default: `false`) | Temporarily disable this interface (brings it down). |
| `zone` | `string` | No | Assign this interface to a security zone. |
| `ipv4` | `list(string)` | No | Static IPv4 addresses in CIDR notation. |
| `ipv6` | `list(string)` | No | Static IPv6 addresses in CIDR notation. |
| `dhcp` | `bool` | No (default: `false`) | Enable DHCP client on this interface. |
| `dhcp_v6` | `bool` | No (default: `false`) | Enable DHCPv6 client for IPv6 address assignment. |
| `ra` | `bool` | No (default: `false`) | Enable Router Advertisements (for IPv6 server mode). |
| `dhcp_client` | `string` | No | DHCPClient specifies how DHCP client is managed:   - "builtin" (default): Use... |
| `table` | `number` | No | Table specifies the routing table ID for this interface. If set to > 0 (and n... |
| `gateway` | `string` | No | Default gateway for static IPv4 configuration. |
| `gateway_v6` | `string` | No | Default gateway for static IPv6 configuration. |
| `mtu` | `number` | No (default: `1500`) | Maximum Transmission Unit size in bytes. |
| `disable_anti_lockout` | `bool` | No | Anti-Lockout protection (sandbox mode only) When true, implicit accept rules ... |
| `access_web_ui` | `bool` | No | ⚠️ *Deprecated.* Web UI / API Access Deprecated: Use Management block instead |
| `web_ui_port` | `number` | No | ⚠️ *Deprecated.* Deprecated: Use Management block instead Port to map (external) |

## Nested Blocks

### new_zone

Create and assign a new zone inline.

```hcl
new_zone "name" {
  color = "..."
  description = "..."
  interface = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

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

#### match

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

#### services

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

##### port

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

#### management

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

### bond

Bond represents the configuration for a bonding interface.

```hcl
bond {
  mode = "..."
  interfaces = [...]
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No |  |
| `interfaces` | `list(string)` | No |  |

### vlan

VLAN represents a VLAN configuration nested within an interface.

```hcl
vlan "id" {
  description = "..."
  zone = "..."
  ipv4 = [...]
  # ...
}
```

**Labels:**

- `id` (required) -

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | `string` | No |  |
| `zone` | `string` | No |  |
| `ipv4` | `list(string)` | No |  |
| `ipv6` | `list(string)` | No |  |

#### new_zone

Create zone inline

```hcl
new_zone "name" {
  color = "..."
  description = "..."
  interface = "..."
  # ...
}
```

**Labels:**

- `name` (required) -

**Attributes:**

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

##### match

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

##### services

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

###### port

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

##### management

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

### management

Management Access (Interface specific overrides)

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

### tls

TLS/Certificate configuration for this interface

```hcl
tls {
  mode = "self-signed"
  hostname = "..."
  email = "..."
  # ...
}
```

**Attributes:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | `string` | No | "self-signed", "acme", "tailscale", "manual" Values: `self-signed`, `manual` |
| `hostname` | `string` | No | For Tailscale mode |
| `email` | `string` | No | ACME (Let's Encrypt) settings |
| `domains` | `list(string)` | No |  |
| `cert_file` | `string` | No | Manual certificate (bring your own) |
| `key_file` | `string` | No |  |
