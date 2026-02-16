# Flywall Configuration Examples

This directory contains example configuration files for Flywall Firewall.
You can use these files to learn about HCL syntax, test the configuration validator, or as a base for your own setup.

## Examples

### Basic & Routing

| File | Description | Features |
|------|-------------|----------|
| [basic.hcl](basic.hcl) | Simple Home Router | WAN/LAN, DHCP Client/Server, DNS, Masquerade |
| [port-forward.hcl](port-forward.hcl) | Service Exposure | DNAT (Port Forwarding), Source Restriction, Range Mapping |
| [complex-routing.hcl](complex-routing.hcl) | Enterprise Routing | BGP, OSPF, Routing Tables, Mark Rules, Traffic Isolation |
| [wildcard-policies.hcl](wildcard-policies.hcl) | Zone Wildcards | Glob patterns in policies (`vpn*`, `*`) |

### VPN & Multi-WAN

| File | Description | Features |
|------|-------------|----------|
| [vpn-failover.hcl](vpn-failover.hcl) | Multi-WAN & Failover | Uplink Groups, Failover, Policy Routing, UID Routing |
| [vpn-wireguard-tailscale.hcl](vpn-wireguard-tailscale.hcl) | VPN Integration | WireGuard Server, Tailscale, VPN Zones, Management Access |

### Security & Monitoring

| File | Description | Features |
|------|-------------|----------|
| [ipset_firehol.hcl](ipset_firehol.hcl) | Threat Blocklists | FireHOL Lists, IPSets, Protection Settings |
| [security-monitoring.hcl](security-monitoring.hcl) | Security Features | Rule Learning, Anomaly Detection, GeoIP, Notifications, Audit |

### Services & Scheduling

| File | Description | Features |
|------|-------------|----------|
| [services-advanced.hcl](services-advanced.hcl) | Network Services | DNS (DoH/DoT), QoS, mDNS, UPnP, NTP, DDNS, Syslog |
| [scheduled-rules.hcl](scheduled-rules.hcl) | Time-Based Rules | Scheduled Rules, Parental Controls, Day/Time Restrictions |

### System & Administration

| File | Description | Features |
|------|-------------|----------|
| [system-api-web.hcl](system-api-web.hcl) | System Config | API Server, Web UI, System Tuning, Feature Flags, HA Replication |

## Usage

### 1. Validate Configuration
Check the syntax and see a summary of the configuration without applying it:

```bash
# Basic check
flywall check examples/basic.hcl

# Detailed summary (Interfaces, Zones, Policies)
flywall check -v examples/vpn-failover.hcl
```

### 2. Preview Ruleset
See the nftables rules that would be generated from a configuration:

```bash
# Dump raw rules
flywall show examples/port-forward.hcl

# Show summary + rules
flywall show --summary examples/complex-routing.hcl
```

### 3. Run/Apply
To run one of these examples (requires root privileges):

```bash
# Run in foreground (Control Plane)
sudo flywall ctl examples/basic.hcl
```
