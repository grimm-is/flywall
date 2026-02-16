---
title: "Upgrading"
linkTitle: "Upgrading"
weight: 30
description: >
  Upgrade Flywall to a new version safely.
---

Flywall supports seamless upgrades with zero-downtime socket handoff. This guide covers upgrade procedures for different installation methods.

## Seamless Upgrade (Recommended)

Flywall can upgrade itself without dropping connections:

```bash
# Download new version
curl -LO https://github.com/grimm-is/flywall/releases/latest/download/flywall-linux-amd64

# Place in standard upgrade location
sudo mv flywall-linux-amd64 /usr/local/bin/flywall_new
sudo chmod +x /usr/local/bin/flywall_new

# Trigger seamless upgrade
sudo flywall upgrade
```

The upgrade process:
1. New binary validates its own configuration
2. Old process hands off listening sockets
3. New process takes over without connection drops
4. Old process exits gracefully

## Standard Upgrade

For a traditional upgrade with brief downtime:

```bash
# Stop Flywall
sudo systemctl stop flywall

# Replace binary
sudo mv flywall-linux-amd64 /usr/local/bin/flywall
sudo chmod +x /usr/local/bin/flywall

# Start Flywall
sudo systemctl start flywall
```

## Configuration Migration

When upgrading between major versions, your configuration may need migration:

```bash
# Check if migration is needed
flywall migrate check -c /etc/flywall/flywall.hcl

# Perform migration (creates backup automatically)
flywall migrate -c /etc/flywall/flywall.hcl
```

The migrate command:
- Backs up your config to `/etc/flywall/flywall.hcl.backup.<timestamp>`
- Updates deprecated fields to new equivalents
- Reports any manual changes required

## Version Compatibility

| From Version | To Version | Migration Required |
|--------------|------------|-------------------|
| 0.1.x | 0.2.x | No |
| 0.2.x | 0.3.x | Auto-migrate |

## Rollback

If an upgrade causes issues:

```bash
# Stop Flywall
sudo systemctl stop flywall

# Restore previous binary (if you kept it)
sudo mv /usr/local/bin/flywall.old /usr/local/bin/flywall

# Restore configuration backup
sudo cp /etc/flywall/flywall.hcl.backup.<timestamp> /etc/flywall/flywall.hcl

# Start Flywall
sudo systemctl start flywall
```

## Checking Version

```bash
flywall version
# Output: flywall 0.2.0 (commit: abc1234, built: 2024-01-15)
```

## Upgrade via Web UI

The web UI also supports upgrades when a new binary is available:

1. Navigate to **System** → **Upgrade**
2. Upload the new binary or provide a URL
3. Click **Upgrade Now**
4. The UI will reconnect automatically after the upgrade
