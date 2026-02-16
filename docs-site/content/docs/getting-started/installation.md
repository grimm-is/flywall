---
title: "Installation"
linkTitle: "Installation"
weight: 10
description: >
  Download and install Flywall on your Linux system.
---

## Requirements

- **OS**: Linux (kernel 5.4+ recommended for all features)
- **Architecture**: amd64 or arm64
- **Privileges**: Root access required for network management

## Quick Install

Download the latest release and install:

```bash
# Download latest release
curl -LO https://github.com/grimm-is/flywall/releases/latest/download/flywall-linux-amd64

# Make executable and move to PATH
chmod +x flywall-linux-amd64
sudo mv flywall-linux-amd64 /usr/local/bin/flywall

# Verify installation
flywall version
```

## Package Installation

### Debian/Ubuntu (coming soon)

```bash
# Add repository
curl -fsSL https://repo.flywall.dev/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/flywall.gpg
echo "deb [signed-by=/etc/apt/keyrings/flywall.gpg] https://repo.flywall.dev/apt stable main" | sudo tee /etc/apt/sources.list.d/flywall.list

# Install
sudo apt update
sudo apt install flywall
```

### Alpine Linux

```bash
# Coming soon - Flywall is developed on Alpine
apk add flywall
```

## Systemd Service

Create a systemd service for automatic startup:

```bash
sudo tee /etc/systemd/system/flywall.service << 'EOF'
[Unit]
Description=Flywall Firewall & Router
After=network.target

[Service]
Type=notify
ExecStart=/usr/local/bin/flywall start
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=no
ProtectSystem=strict
ReadWritePaths=/opt/flywall/var/lib /opt/flywall/var/log

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable flywall
```

## Directory Structure

Flywall follows the Filesystem Hierarchy Standard:

| Path | Purpose |
|------|---------|
| `/opt/flywall/etc/flywall.hcl` | Main configuration file |
| `/opt/flywall/var/lib/` | State directory (leases, keys, database) |
| `/opt/flywall/var/log/` | Log files |
| `/usr/local/bin/flywall` | Binary (or `/usr/sbin/flywall`) |

## Verify Installation

After installation, verify Flywall is working:

```bash
# Check version
flywall version

# Validate configuration (create a minimal config first)
flywall validate -c /opt/flywall/etc/flywall.hcl

# Start in foreground for testing
sudo flywall start -c /opt/flywall/etc/flywall.hcl
```

## Next Steps

- [Quickstart]({{< relref "quickstart" >}}) - Create your first configuration
- [Upgrading]({{< relref "upgrading" >}}) - Upgrade from a previous version
