# Flywall Features

Feature maturity levels based on integration test coverage.

| Level | Meaning |
|:-----:|---------|
| ✅ L5 | Production-ready, comprehensive tests |
| 🟩 L4 | Integration tested in VM |
| 🟨 L3 | Works, limited testing |
| 🟧 L2 | Scaffolded, may not function |
| 🔲 L1 | Config only, no runtime |
| ⬜ L0 | Not started |

---

## Core Networking

| Feature | Level | Notes | Docs |
|---------|:-----:|-------|------|
| Zone-based Firewall | ✅ | Policies, stateful tracking, nftables | [Guide](https://docs.flywall.dev/docs/guides/firewall-policies/) |
| nftables Generation | 🟩 | Atomic apply with rollback | |
| Interface Management | 🟩 | Static IP, DHCP client | |
| VLAN / Bonding | 🟩 | Full support via netlink | |
| Static Routing | 🟩 | IPv4/IPv6 routes | |
| Policy Routing | 🟩 | fwmark-based routing tables | |
| NAT & Port Forwarding | 🟩 | Masquerade, DNAT, hairpin NAT | [Guide](https://docs.flywall.dev/docs/guides/nat-port-forwarding/) |
| HCL Configuration | ✅ | Validation, migration, hot reload | [Reference](https://docs.flywall.dev/docs/configuration/) |

## Network Services

| Feature | Level | Notes | Docs |
|---------|:-----:|-------|------|
| DHCP Server | 🟩 | Leases, persistence, static reservations | [Guide](https://docs.flywall.dev/docs/guides/dhcp-dns/) |
| DNS Resolver | 🟩 | Caching, blocklists, split-horizon | [Guide](https://docs.flywall.dev/docs/guides/dhcp-dns/) |
| DNS Egress Control | 🟩 | "DNS Wall" - blocks non-resolved IPs | |
| DNS over HTTPS/TLS | 🟩 | DoH, DoT, DNSSEC validation | |
| Wake-on-LAN | 🟩 | Magic packet sending | |
| mDNS Reflector | 🟩 | Cross-VLAN Bonjour/Avahi | |
| UPnP/NAT-PMP | 🟩 | Automatic port forwarding | |
| Router Advertisements | 🟩 | IPv6 SLAAC | |
| LLDP Discovery | 🟩 | Switch/device detection | |
| Threat Intel Integration | 🟩 | FireHOL, URLhaus blocklists | |

## Security

| Feature | Level | Notes | Docs |
|---------|:-----:|-------|------|
| Privilege Separation | ✅ | ctl(root) / api(unprivileged) | [Architecture](https://docs.flywall.dev/docs/reference/architecture/) |
| Network Namespace Sandbox | 🟩 | API runs in isolated netns | |
| Integrity Monitor | 🟩 | Auto-restore on nftables tampering | |
| Smart Flush | 🟩 | Dynamic sets persist across reloads | |
| Fail2Ban-style Blocking | 🟩 | Automatic brute-force protection | |
| IPSet Blocklists | 🟩 | URL-fetched threat lists | |
| SYN Flood Protection | 🟩 | Rate limiting, SYN cookies | |
| Time-of-Day Rules | 🟩 | Schedule-based policies (kernel 5.4+) | |
| GeoIP Filtering | 🔲 | Config only, runtime planned | |

## VPN

| Feature | Level | Notes | Docs |
|---------|:-----:|-------|------|
| WireGuard | 🟩 | Native via netlink/wgctrl | [Guide](https://docs.flywall.dev/docs/guides/wireguard/) |
| Tailscale Integration | 🟩 | Status/control via socket | |
| VPN Lockout Protection | 🟩 | Prevents config-breaking changes | |

## API & User Interface

| Feature | Level | Notes | Docs |
|---------|:-----:|-------|------|
| REST API | 🟩 | Full CRUD for all resources | [Reference](https://docs.flywall.dev/docs/reference/api/) |
| WebSocket Events | 🟩 | Real-time updates | |
| OpenAPI / Swagger | 🟩 | Interactive API docs | |
| Web Dashboard | 🟨 | Most pages functional | [Guide](https://docs.flywall.dev/docs/guides/web-ui/) |
| TLS / Authentication | 🟩 | API keys, session cookies | |

## Operations

| Feature | Level | Notes | Docs |
|---------|:-----:|-------|------|
| Hot Reload | 🟩 | SIGHUP or API call | |
| Atomic Apply | 🟩 | Rollback on failure | |
| Seamless Upgrade | 🟩 | Socket handoff, zero downtime | [Guide](https://docs.flywall.dev/docs/getting-started/upgrading/) |
| Prometheus Metrics | 🟩 | /metrics endpoint | |
| Syslog Forwarding | 🟩 | Remote logging | |
| Multi-WAN Failover | 🟨 | Health checks, failover | [Guide](https://docs.flywall.dev/docs/guides/multi-wan-failover/) |
| HA Replication | 🟨 | DB sync + custom failover | |

## Learning Engine

| Feature | Level | Notes |
|---------|:-----:|-------|
| Flow Tracking | 🟩 | nflog-based connection logging |
| SNI Snooping | 🟩 | HTTPS destination identification |
| Pending Rule Approval | 🟩 | Review before allowing new flows |
| Device Discovery | 🟩 | DHCP + ARP fingerprinting |

---

## Summary

| Level | Count | Description |
|:-----:|:-----:|-------------|
| ✅ L5 | 3 | Production-ready |
| 🟩 L4 | 38 | Integration tested |
| 🟨 L3 | 4 | Functional, limited tests |
| 🔲 L1 | 1 | Config only |

**Total Features**: 46

---

📖 **Full Documentation**: [docs.flywall.dev](https://docs.flywall.dev)
