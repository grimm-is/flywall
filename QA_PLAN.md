# QA Strategy & Test Plan

## Overview
This document outlines the comprehensive Quality Assurance strategy for Flywall. The primary goal is to verify that the system is not only correct but also **"grokable"** — ensuring that users can intuitively configure the system via the Web UI, TUI, and plain text configuration files.

## Philosophy
*   **User-Story Driven**: All tests should be traceable back to a user story (e.g., "As a user, I want to block an IP address").
*   **Triple-Interface Verification**: Each core feature must be verifiable across all three interfaces:
    1.  **Web UI**: The primary graphical interface.
    2.  **TUI**: The terminal user interface (`flywall console`).
    3.  **Config**: The "Infrastructure as Code" layer (`.hcl` files).
*   **Automated First**: Manual testing is a fallback. We aim for automated regression testing for all interfaces.
*   **Demo VM as Backend**: All automated tests (Web, TUI, API) run against the `flywall.sh demo` or `test-demo` environment for a clean, realistic test bed.

## Test Pyramid
1.  **Unit Tests** (`go test`): Verify internal logic (e.g., HCL parsing, IP validation, nftables script generation).
2.  **Integration Tests** (`fw test int`): Verify system behavior in a VM (QEMU/Orca). Focus on runtime correctness (kernel programming, network connectivity).
3.  **End-to-End (E2E) Tests**:
    *   **UI E2E**: Playwright tests interacting with the Web Dashboard.
    *   **TUI E2E**: Automated interaction with the `flywall console`.
    *   **Config E2E**: File-watcher tests (Edit File -> Assert System State).

---

## Integration Test Audit

The following is an inventory of existing integration tests, mapped by category.

| Category | Directory | Tests | Coverage Notes |
|---|---|---|---|
| **Sanity** | `00-sanity/` | 16 | Combined test covering DHCP, DNS, NAT, Interfaces, Policies, Protections, Routing, nftables, IPSets, IPv6. |
| **Golang Unit** | `05-golang/` | 1 | Runs `go test` inside the VM. |
| **API** | `10-api/` | 18 | Auth, CSRF, CRUD, Staging, Backup, Debug, OpenAPI, etc. |
| **DHCP** | `20-dhcp/` | 4 | Exhaustion, Lifecycle, Options, Traffic. |
| **DNS** | `25-dns/` | 8 | Blocklists, Conditional, Dynamic, Query Log, Split Horizon, Encrypted. |
| **Firewall** | `30-firewall/` | 6 | Integrity, Fail-open, Rulegen, Smart Flush, Zones, Stats. |
| **Network** | `40-network/` | 15 | Bonding, Conntrack, ICMP, Interface Deps, mDNS, Multi-WAN, NAT Traffic, Safe Apply, Packet Flow, RA, UID Routing, Uplink, UPnP, VLAN. |
| **Security** | `50-security/` | 13 | Atomic Apply, Fail2Ban, FireHOL, IPSet Traffic, Learning Traffic, Netns, Network Learning, Pending Rules, Port Scan, Protection Traffic, SNI Snoop, Threat Intel. |
| **VPN** | `60-vpn/` | 9 | Tailscale status/control, VPN Failure, Isolation, Lockout, WireGuard, Client Import. |
| **System** | `70-system/` | 27 | Boot Loop, CLI, Config Reload, DDNS, Device Discovery/Identity, Feature Flags, HA Full Stack/Partition, Import, LLDP, MAC Vendor, Management Granularity, Persistence, Plugin, Reload, Replication, Schema Migration/Versioning, Service Access, State Persistence, Upgrade. |
| **Monitoring** | `80-monitoring/` | 6 | Alerting, Analytics, Metrics Endpoint, Monitor, NFLog Capture, Syslog. |
| **Scenarios** | `99-scenarios/` | 2 | Container Host, Personal Firewall (endpoint mode). |

**Total: ~124 Integration Tests**

---

## Extended User Stories

Below is an expanded list of user stories, organized by persona and feature area.

### Network Administrator (SOHO / SMB)

| # | User Story | Web UI | TUI | HCL Config | Int Test Coverage |
|---|---|---|---|---|---|
| 1 | I want to set my LAN IP and WAN interface | `Interfaces` page | Interface screen | `interface {}` block | ✅ `interface_test.sh` |
| 2 | I want to create a VLAN for my IoT devices | `Network` -> `VLANs` | N/A | `interface "vlan10" {}` | ✅ `vlan_test.sh` |
| 3 | I want to bond two interfaces for redundancy | `Network` -> `Bonds` | N/A | `bond {}` block | ✅ `bond_test.sh` |
| 4 | I want to define security zones (LAN, WAN, Guest) | `Zones` page | Zones view | `zone {}` block | ✅ `zones_test.sh` |
| 5 | I want to block traffic from a specific IP | `Policies` -> Add Rule | Rules view | `rule "drop" {}` | ✅ `policy_test.sh` |
| 6 | I want to allow only port 443 from WAN to LAN | `Policies` -> Add Rule | Rules view | `rule "accept" { dest_port = 443 }` | ✅ `policy_test.sh` |
| 7 | I want to forward port 80 to my internal webserver | `NAT` -> Port Forward | NAT view | `nat "dnat" {}` | ✅ `nat_test.sh` |
| 8 | I want hairpin NAT so LAN can reach my public IP | `NAT` -> Hairpin | N/A | `hairpin = true` | ✅ `nat_traffic_test.sh` |
| 9 | I want to reserve a static DHCP IP for my printer | `DHCP` -> Reservations | DHCP view | `static_lease {}` | ✅ `dhcp_test.sh` |
| 10 | I want to serve DNS to my LAN with caching | `DNS` -> Serve Zone | N/A | `dns { serve {} }` | ✅ `dns_test.sh` |
| 11 | I want to block ads using a DNS blocklist | `DNS` -> Blocklists | N/A | `blocklist {}` | ✅ `dns_blocklist_*.sh` |
| 12 | I want Split Horizon DNS (internal override) | `DNS` -> Overrides | N/A | `override {}` | ✅ `split_horizon_test.sh` |
| 13 | I want to use DoH/DoT for upstream | `DNS` -> Forwarders | N/A | `forwarders = ["https://..."]` | ✅ `encrypted_dns_test.sh` |
| 14 | I want to add a WireGuard peer | `VPN` -> WireGuard | WireGuard view | `wireguard_peer {}` | ✅ `vpn_test.sh` |
| 15 | I want to import a WireGuard client config | `VPN` -> Import | N/A | `POST /api/vpn/import` | ✅ `wireguard-client-import.sh` |
| 16 | I want Tailscale integration | `VPN` -> Tailscale | N/A | `tailscale {}` | ✅ `tailscale_*.sh` |
| 17 | I want Multi-WAN failover | `Network` -> Uplinks | N/A | `route_group {}` | ✅ `multi_wan_test.sh` |
| 18 | I want to manually switch WAN uplink | `Uplinks` -> Switch | N/A | `POST /api/uplinks/switch` | ✅ `uplink_api_test.sh` |
| 19 | I want to schedule rules (time-of-day) | `Scheduler` -> Add | N/A | `schedule {}` | ✅ `scheduler_api_test.sh` |
| 20 | I want to enable mDNS reflection | `Services` -> mDNS | N/A | `mdns { enabled = true }` | ✅ `mdns_test.sh` |
| 21 | I want UPnP/NAT-PMP for gaming | `Services` -> UPnP | N/A | `upnp { enabled = true }` | ✅ `upnp_test.sh` |
| 22 | I want IPv6 (RA/SLAAC) | `Network` -> IPv6 | N/A | `router_advertisement {}` | ✅ `ra_test.sh`, `ipv6_test.sh` |

### Security-Focused User

| # | User Story | Web UI | TUI | HCL Config | Int Test Coverage |
|---|---|---|---|---|---|
| 23 | I want to block brute-force SSH attempts | `Protections` -> Fail2Ban | N/A | `protection "fail2ban" {}` | ✅ `fail2ban_test.sh` |
| 24 | I want to block IPs from FireHOL/Threat Intel | `IPSets` -> Blocklist | N/A | `ipset "firehol" {}` | ✅ `firehol_test.sh`, `threat_intel_test.sh` |
| 25 | I want SYN flood protection | `Protections` -> SYN Limit | N/A | `protection "syn_limit" {}` | ✅ `protection_test.sh` |
| 26 | I want port-scan detection | Logs -> Alerts | N/A | `protection "port_scan" {}` | ✅ `port_scan_test.sh` |
| 27 | I want to see who's connecting (Learning Mode) | `Firewall` -> Learning | Learning view | `rule_learning { enabled = true }` | ✅ `learning_traffic_test.sh` |
| 28 | I want to approve/deny a learned rule | `Learning` -> Approve | Learning view | `POST /api/learning/rules/{id}/approve` | ✅ `pending_rules_test.sh` |
| 29 | I want SNI snooping for HTTPS dest | N/A (internal) | N/A | - | ✅ `sni_snoop_test.sh` |
| 30 | I want to block all traffic except DNS-authorized | `DNS Wall` | N/A | `dns_wall { enabled = true }` | 🔲 (Future) |

### Operations / DevOps

| # | User Story | Web UI | TUI | HCL Config | Int Test Coverage |
|---|---|---|---|---|---|
| 31 | I want to backup my configuration | `System` -> Backup | N/A | `flywall backup` | ✅ `backup_api_test.sh` |
| 32 | I want to restore a previous config | `Backup` -> Restore | N/A | `flywall restore` | ✅ `backup_api_test.sh` |
| 33 | I want to hot-reload config without restart | N/A | N/A | `flywall reload` | ✅ `reload_test.sh` |
| 34 | I want Safe Apply (auto-rollback on loss) | `Config` -> Apply Safe | N/A | `POST /api/config/safe-apply` | ✅ `network_safe_apply_test.sh` |
| 35 | I want seamless upgrade (zero-downtime) | `System` -> Upgrade | N/A | `flywall upgrade` | ✅ `upgrade_test.sh` |
| 36 | I want HA (Active/Standby replication) | `System` -> HA | N/A | `ha {}` block | ✅ `ha_full_stack_test.sh`, `replication_test.sh` |
| 37 | I want Prometheus metrics | `/metrics` | N/A | - | ✅ `metrics_endpoint_test.sh` |
| 38 | I want Syslog forwarding | `Logging` -> Syslog | N/A | `syslog {}` | ✅ `syslog_test.sh` |
| 39 | I want alerting on events | `Alerts` -> Rules | N/A | `alerting {}` | ✅ `alerting_test.sh` |
| 40 | I want to import a pfSense/OPNsense config | `System` -> Import | N/A | `flywall import` | ✅ `import_test.sh` |

### Discovery / Visibility

| # | User Story | Web UI | TUI | HCL Config | Int Test Coverage |
|---|---|---|---|---|---|
| 41 | I want to discover devices on my network | `Network` -> Devices | Devices view | - | ✅ `device_discovery_test.sh` |
| 42 | I want to label a device (alias, owner) | `Devices` -> Edit | Devices view | `POST /api/devices/identity` | ✅ `device_identity_test.sh` |
| 43 | I want to see MAC vendor (fingerprinting) | `Devices` -> Info | N/A | - | ✅ `mac_vendor_test.sh` |
| 44 | I want to scan a subnet for open ports | `Scanner` -> Run | N/A | `POST /api/scanner/network` | 🔲 (Gap) |
| 45 | I want LLDP neighbor discovery | `Network` -> LLDP | N/A | `lldp { enabled = true }` | ✅ `lldp_test.sh` |

### Endpoint / Personal Firewall (Scenario)

| # | User Story | Web UI | TUI | HCL Config | Int Test Coverage |
|---|---|---|---|---|---|
| 46 | I want to run Flywall on my laptop (stealth) | N/A | N/A | `policy "wan" "firewall" { action = "drop" }` | ✅ `scenario_personal_firewall_test.sh` |
| 47 | I want to allow only admin IP to SSH | N/A | N/A | `rule { src_ip = "admin_ip" }` | ✅ `scenario_personal_firewall_test.sh` |

---

## Gap Analysis (Tests Needed)

| Feature | User Story | Gap Type | Action |
|---|---|---|---|
| Network Scanner | Scan subnet for hosts | **No Integration Test** | Add `scanner_test.sh` |
| DNS Wall | Block non-DNS-authorized IPs | **Feature Pending** | Implement + Test |
| TUI Automation | All TUI scenarios | **No E2E Tests** | Scaffold `tests/tui/` |
| Device Groups | Create/manage device groups | **UI Test Gap** | Add Playwright spec |

---

## Automation Implementation Plan

### 1. Web UI Automation (Playwright)
*   **Location**: `ui/tests/e2e`
*   **Backend**: `flywall.sh test-demo` (ephemeral Demo VM).
*   **Expand**: `playwright.demo.config.ts` to cover Learning, Devices, Scanner.

### 2. TUI Automation (teatest / go-expect)
*   **Location**: `tests/tui/` (New)
*   **Backend**: Demo VM (`localhost:$DEMO_PORT`).
*   **Scenarios**: Dashboard status, Policy view, Learning approve, Device list.

### 3. Config/API Automation (Integration Suite)
*   **Location**: `integration_tests/linux/`
*   **New Tests**:
    *   `10-api/scanner_test.sh`
    *   `50-security/learning_approval_e2e_test.sh`

---

## Next Steps
1.  [x] **Audit**: Complete integration test inventory.
2.  [ ] **Scanner Test**: Add `scanner_test.sh` for subnet scanning.
3.  [ ] **TUI PoC**: Scaffold `tests/tui/` with `teatest` harness.
4.  [ ] **UI Gaps**: Add Playwright specs for Learning, Devices.
5.  [ ] **TUI Gaps**: Bring the TUI up to parity with the Web UI.
