---
title: "CLI Reference"
linkTitle: "CLI"
weight: 10
description: >
  Flywall command-line interface reference.
---

The `flywall` binary provides all functionality for managing the firewall.

## Global Options

```bash
flywall [global options] <command> [command options]
```

| Option | Description |
|--------|-------------|
| `-c, --config <path>` | Configuration file (default: `/opt/flywall/etc/flywall.hcl`) |
| `-v, --verbose` | Verbose output |
| `--version` | Show version information |
| `-h, --help` | Show help |

---

## Commands

### start

Start the Flywall daemon.

```bash
flywall start [options]
```

| Option | Description |
|--------|-------------|
| `-c, --config` | Path to config file |
| `--foreground` | Run in foreground (don't daemonize) |
| `--debug` | Enable debug logging |

**Examples:**
```bash
# Start with default config
sudo flywall start

# Start with custom config in foreground
sudo flywall start -c /path/to/config.hcl --foreground
```

---

### stop

Stop the running Flywall daemon.

```bash
flywall stop
```

---

### reload

Reload configuration without restart (hot reload).

```bash
flywall reload
```

Or send SIGHUP:
```bash
kill -HUP $(cat /var/run/flywall.pid)
```

---

### validate

Validate configuration file syntax and semantics.

```bash
flywall validate [options]
```

| Option | Description |
|--------|-------------|
| `-c, --config` | Path to config file |
| `--strict` | Treat warnings as errors |

**Examples:**
```bash
flywall validate -c /etc/flywall/flywall.hcl

# Exit codes:
# 0 - Valid
# 1 - Invalid
```

---

### upgrade

Perform seamless upgrade with socket handoff.

```bash
flywall upgrade [options]
```

| Option | Description |
|--------|-------------|
| `--binary <path>` | Path to new binary (default: `flywall_new`) |
| `--timeout <duration>` | Handoff timeout (default: 30s) |

**Example:**
```bash
# Place new binary and upgrade
sudo cp flywall-new /usr/local/bin/flywall_new
sudo flywall upgrade
```

---

### status

Show daemon status and key metrics.

```bash
flywall status [options]
```

| Option | Description |
|--------|-------------|
| `--json` | Output as JSON |

**Example output:**
```
Flywall Status
  Version: 0.2.0
  Uptime: 3d 14h 22m
  Config: /etc/flywall/flywall.hcl

Services:
  Firewall: active (2,847 rules)
  DHCP: active (45 leases)
  DNS: active (15,234 queries cached)
  VPN: active (2 peers connected)
```

---

### version

Show version information.

```bash
flywall version
```

Output:
```
flywall 0.2.0 (commit: a1b2c3d, built: 2024-01-15T10:30:00Z)
```

---

## Subsystem Commands

### dhcp

Manage DHCP service.

```bash
flywall dhcp <subcommand>
```

| Subcommand | Description |
|------------|-------------|
| `leases` | List active leases |
| `release <ip>` | Release a specific lease |
| `reserve <mac> <ip>` | Add a reservation |

**Examples:**
```bash
# List leases
flywall dhcp leases

# Release a lease
flywall dhcp release 192.168.1.105
```

---

### dns

Manage DNS service.

```bash
flywall dns <subcommand>
```

| Subcommand | Description |
|------------|-------------|
| `flush` | Clear DNS cache |
| `stats` | Show DNS statistics |
| `lookup <domain>` | Query a domain |
| `test-upstream` | Test upstream connectivity |

**Examples:**
```bash
flywall dns flush
flywall dns stats
flywall dns lookup google.com
```

---

### wan

Manage multi-WAN configuration.

```bash
flywall wan <subcommand>
```

| Subcommand | Description |
|------------|-------------|
| `status` | Show WAN status and health |
| `health` | Show health check details |
| `failover <interface>` | Force failover to interface |
| `history` | Show failover history |

---

### vpn

Manage VPN connections.

```bash
flywall vpn <subcommand>
```

| Subcommand | Description |
|------------|-------------|
| `status` | Show VPN status |
| `peers` | List connected peers |
| `disconnect <peer>` | Disconnect a peer |

---

### debug

Debugging utilities.

```bash
flywall debug <subcommand>
```

| Subcommand | Description |
|------------|-------------|
| `trace` | Real-time packet tracing |
| `rules` | Dump generated nftables rules |
| `config` | Show parsed configuration |

**Examples:**
```bash
# Trace HTTPS traffic
flywall debug trace --proto tcp --dport 443

# Show generated rules
flywall debug rules
```

---

## Configuration

### migrate

Migrate configuration between versions.

```bash
flywall migrate [options]
```

| Option | Description |
|--------|-------------|
| `-c, --config` | Path to config file |
| `--check` | Check if migration needed (don't apply) |
| `--backup` | Create backup before migration (default: true) |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Permission denied |
| 4 | Already running |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `FLYWALL_CONFIG` | Default config file path |
| `FLYWALL_LOG_LEVEL` | Logging level (debug, info, warn, error) |
| `FLYWALL_STATE_DIR` | State directory override |
