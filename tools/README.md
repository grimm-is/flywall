# Flywall

<p align="center">
  <strong>Modern Linux Firewall & Router</strong><br>
  Single-binary, zone-based firewall with embedded DHCP, DNS, and VPN.
</p>

<p align="center">
  <a href="https://docs.flywall.dev">Documentation</a> •
  <a href="https://docs.flywall.dev/docs/getting-started/">Getting Started</a> •
  <a href="FEATURES.md">Features</a>
</p>

---

## What is Flywall?

Flywall is a single-binary Linux firewall and router built in Go. It replaces complex network stacks (iptables + dnsmasq + isc-dhcp + wireguard-tools) with one unified, easy-to-configure daemon.

### Key Features

- **Zone-based Firewall** — Define security zones and policies; Flywall generates optimized nftables rules
- **Embedded Services** — Built-in DHCP server, DNS resolver with caching/blocklists, and native WireGuard VPN
- **Modern Web UI** — Real-time dashboard for monitoring and configuration (Svelte 5)
- **HCL Configuration** — Human-readable config with validation, hot reload, and atomic apply
- **Privilege Separation** — Sandboxed API server with root-only control plane

### Current Status

**v0.2** — Functional for home lab and enthusiast use. See [FEATURES.md](FEATURES.md) for detailed maturity levels.

---

## Quick Start

### 1. Install

```bash
# Download latest release
curl -LO https://github.com/grimm-is/flywall/releases/latest/download/flywall-linux-amd64
chmod +x flywall-linux-amd64
sudo mv flywall-linux-amd64 /usr/local/bin/flywall

# Verify
flywall version
```

### 2. Configure

Create `/opt/flywall/etc/flywall.hcl`:

```hcl
ip_forwarding = true

interface "eth0" { zone = "WAN"; dhcp = true }
interface "eth1" { zone = "LAN"; ipv4 = ["192.168.1.1/24"] }

zone "LAN" { management { web_ui = true } }

policy "LAN" "WAN" {
  rule "allow" { action = "accept" }
}

nat "outbound" { type = "masquerade"; out_interface = "eth0" }

dhcp {
  scope "lan" {
    interface   = "eth1"
    range_start = "192.168.1.100"
    range_end   = "192.168.1.200"
  }
}

dns { forwarders = ["1.1.1.1", "8.8.8.8"] }
web { listen = ":8080" }
```

### 3. Run

```bash
sudo flywall start -c /opt/flywall/etc/flywall.hcl
```

Open http://192.168.1.1:8080 for the web dashboard.

📖 **Full guide**: [docs.flywall.dev/docs/getting-started/quickstart/](https://docs.flywall.dev/docs/getting-started/quickstart/)

---

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│  flywall api (nobody, sandboxed)   HTTP:8080 / HTTPS:8443  │
│  └─ REST API + WebSocket + Web UI                          │
├────────────────────────────────────────────────────────────┤
│  flywall ctl (root)                Unix Socket RPC          │
│  └─ Firewall │ Network │ DHCP │ DNS │ VPN │ Learning       │
└────────────────────────────────────────────────────────────┘
```

---

## Documentation

- 📚 **[Full Documentation](https://docs.flywall.dev)** — Comprehensive guides and reference
- 🚀 **[Getting Started](https://docs.flywall.dev/docs/getting-started/)** — Installation, quickstart, upgrading
- 📖 **[Guides](https://docs.flywall.dev/docs/guides/)** — Firewall, DHCP/DNS, NAT, VPN, Multi-WAN
- 🔧 **[Configuration Reference](https://docs.flywall.dev/docs/configuration/)** — All HCL options
- 🔌 **[API Reference](https://docs.flywall.dev/docs/reference/api/)** — REST API documentation

---

## Development

```bash
# Setup (requires direnv)
direnv allow

# Build
fw build

# Run tests
fw test int

# Development VM
fw dev
```

See [docs/dev/](docs/dev/) for developer documentation.

---

## Contributing

Contributions are welcome! Please read the developer docs before submitting PRs.

- Report bugs via [GitHub Issues](https://github.com/grimm-is/flywall/issues)
- Discuss ideas in [GitHub Discussions](https://github.com/grimm-is/flywall/discussions)

---

## License

[AGPL-3.0](LICENSE) — Flywall is free software that must remain free.
