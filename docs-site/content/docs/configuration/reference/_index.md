---
title: "Configuration Reference"
linkTitle: "Reference"
weight: 10
description: >
  Complete reference of all HCL configuration options.
---

This reference is auto-generated from the Flywall source code.

**Schema Version:** 1.0

## Configuration Blocks

| Block | Description |
|-------|-------------|
| [anomaly_detection]({{< relref "anomaly_detection" >}}) | AnomalyConfig configures traffic anomaly detection. |
| [api]({{< relref "api" >}}) | API configuration |
| [audit]({{< relref "audit" >}}) | Audit logging configuration |
| [cloud]({{< relref "cloud" >}}) | Cloud Management |
| [ddns]({{< relref "ddns" >}}) | Dynamic DNS |
| [dhcp]({{< relref "dhcp" >}}) | DHCPServer configuration. |
| [dns]({{< relref "dns" >}}) | New consolidated DNS config |
| [dns_server]({{< relref "dns_server" >}}) ⚠️ | Deprecated: use DNS |
| [features]({{< relref "features" >}}) | Feature Flags |
| [frr]({{< relref "frr" >}}) | FRRConfig holds configuration for Free Range Routing (FRR). |
| [geoip]({{< relref "geoip" >}}) | GeoIP configuration for country-based filtering |
| [interface]({{< relref "interface" >}}) | Interface represents a physical or virtual network interf... |
| [ipset]({{< relref "ipset" >}}) | IPSet defines a named set of IPs/networks for use in fire... |
| [mark_rule]({{< relref "mark_rule" >}}) | MarkRule represents a rule for setting routing marks on p... |
| [mdns]({{< relref "mdns" >}}) | mDNS Reflector configuration |
| [multi_wan]({{< relref "multi_wan" >}}) | MultiWAN represents multi-WAN configuration for failover ... |
| [nat]({{< relref "nat" >}}) | NATRule defines Network Address Translation rules. |
| [notifications]({{< relref "notifications" >}}) | NotificationsConfig configures the notification system. |
| [ntp]({{< relref "ntp" >}}) | NTP configuration |
| [policy]({{< relref "policy" >}}) | Policy defines traffic rules between zones. Rules are eva... |
| [policy_route]({{< relref "policy_route" >}}) | PolicyRoute represents a policy-based routing rule. Polic... |
| [protection]({{< relref "protection" >}}) | InterfaceProtection defines security protection settings ... |
| [qos_policy]({{< relref "qos_policy" >}}) | Per-interface settings (first-class) |
| [replication]({{< relref "replication" >}}) | State Replication configuration |
| [route]({{< relref "route" >}}) | Route represents a static route configuration. |
| [routing_table]({{< relref "routing_table" >}}) | RoutingTable represents a custom routing table configurat... |
| [rule_learning]({{< relref "rule_learning" >}}) | Rule learning and notifications |
| [scheduled_rule]({{< relref "scheduled_rule" >}}) | ScheduledRule defines a firewall rule that activates on a... |
| [scheduler]({{< relref "scheduler" >}}) | SchedulerConfig defines scheduler settings. |
| [syslog]({{< relref "syslog" >}}) | Syslog remote logging |
| [system]({{< relref "system" >}}) | System tuning and settings |
| [threat_intel]({{< relref "threat_intel" >}}) | ThreatIntel configures threat intelligence feeds. |
| [uid_routing]({{< relref "uid_routing" >}}) | UIDRouting configures per-user routing (for SOCKS proxies... |
| [uplink_group]({{< relref "uplink_group" >}}) | UplinkGroup configures a group of uplinks (WAN, VPN, etc.... |
| [upnp]({{< relref "upnp" >}}) | UPnP IGD configuration |
| [vpn]({{< relref "vpn" >}}) | VPN integrations (Tailscale, WireGuard, etc.) for secure ... |
| [web]({{< relref "web" >}}) | Web Server configuration (previously part of API) |
| [zone]({{< relref "zone" >}}) | Zone defines a network security zone. Zones can match tra... |

## Global Attributes

Top-level configuration attributes are documented in [Global Settings]({{< relref "global" >}}).

## Minimal Example

```hcl
schema_version = "1.0"
ip_forwarding = true

interface "eth0" {
  zone = "WAN"
  dhcp = true
}

interface "eth1" {
  zone = "LAN"
  ipv4 = ["192.168.1.1/24"]
}

zone "WAN" {}
zone "LAN" {
  management { web_ui = true }
}

policy "LAN" "WAN" {
  rule "allow" { action = "accept" }
}
```
