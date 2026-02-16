---
title: "Flywall"
---

{{< blocks/cover title="Flywall" subtitle="Modern Linux Firewall & Router" image_anchor="top" height="full" >}}
<a class="btn btn-lg btn-primary me-3 mb-4" href="{{< relref "/docs/getting-started" >}}">
  Get Started <i class="fas fa-arrow-alt-circle-right ms-2"></i>
</a>
<a class="btn btn-lg btn-secondary me-3 mb-4" href="https://github.com/grimm-is/flywall">
  <i class="fab fa-github me-2"></i> View on GitHub
</a>
<p class="lead mt-5">Single-binary, zone-based firewall with embedded DHCP, DNS, and VPN.</p>
{{< /blocks/cover >}}

{{% blocks/lead color="primary" %}}
**Flywall** is a modern, single-binary Linux firewall and router. It replaces complex stacks
(iptables + dnsmasq + isc-dhcp + wireguard-tools) with one unified, Go-powered daemon.

Configuration is done via HCL files or a polished web UI.
{{% /blocks/lead %}}

{{< blocks/section color="dark" type="row" >}}

{{% blocks/feature icon="fa-shield-alt" title="Zone-Based Firewall" %}}
Define security zones and policies. Flywall generates optimized nftables rulesets automatically.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-network-wired" title="Embedded Services" %}}
Built-in DHCP server, DNS resolver with caching and blocklists, and native WireGuard VPN.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-tachometer-alt" title="Web Dashboard" %}}
Real-time monitoring, configuration editing, and device discovery through a modern Svelte UI.
{{% /blocks/feature %}}

{{< /blocks/section >}}

{{< blocks/section >}}

## Why Flywall?

| Traditional Stack | Flywall |
|-------------------|---------|
| iptables/nftables + firewalld | ✅ Unified zone-based policies |
| dnsmasq or bind9 | ✅ Built-in DNS with DoH/DoT |
| isc-dhcp-server | ✅ Integrated DHCP with UI |
| wireguard-tools | ✅ Native WireGuard via netlink |
| Multiple config files | ✅ Single HCL config |
| Manual scripting | ✅ Hot reload, atomic apply |

{{< /blocks/section >}}

{{< blocks/section color="primary" type="row" >}}

{{% blocks/feature icon="fa-book" title="Documentation" url="/docs/" %}}
Complete guides for installation, configuration, and operation.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-code" title="API Reference" url="/docs/reference/api/" %}}
Full REST API with WebSocket events for automation.
{{% /blocks/feature %}}

{{% blocks/feature icon="fab fa-github" title="Contribute" url="https://github.com/grimm-is/flywall" %}}
Open source under AGPL-3.0. Contributions welcome!
{{% /blocks/feature %}}

{{< /blocks/section >}}
