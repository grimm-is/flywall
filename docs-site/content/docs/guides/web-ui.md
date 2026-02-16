---
title: "Web UI Guide"
linkTitle: "Web UI"
weight: 5
description: >
  Navigate and use the Flywall web dashboard.
---

The Flywall Web UI provides a modern, real-time interface for monitoring and configuring your firewall. Access it at `http://your-flywall-ip:8080` (or HTTPS on port 8443).

## First-Time Setup

When accessing Flywall for the first time, you'll be prompted to create an administrator account:

1. Open your browser to `http://192.168.1.1:8080`
2. Enter a username and password
3. Click **Create Account**

{{% alert title="Security" color="warning" %}}
Use a strong, unique password. This account has full administrative access.
{{% /alert %}}

---

## Dashboard

The **Dashboard** is your at-a-glance overview of system status.

### Widgets

| Widget | Description |
|--------|-------------|
| **Network Topology** | Visual map of interfaces, zones, and connections |
| **System Status** | Uptime, CPU, memory, and disk usage |
| **Uplinks** | WAN connection status with health indicators |
| **Zones** | Security zones with device counts and traffic stats |

### Customizing the Dashboard

1. Click **Customize** in the top-right corner
2. Drag widgets to reorder them
3. Click **Reset** to restore the default layout
4. Click **Done** when finished

---

## Network

### Interfaces

The **Interfaces** page shows all network interfaces with their current state.

#### Interface Cards

Each interface displays:
- **Name** (e.g., `eth0`, `wlan0`)
- **Zone** assignment (WAN, LAN, etc.)
- **IP Addresses** (IPv4/IPv6)
- **Link State** (up/down indicator)
- **Traffic Stats** (TX/RX bytes)

#### Editing an Interface

1. Click on an interface card to expand it
2. Click the **Edit** button (pencil icon)
3. Modify settings:
   - **Zone** - Assign to a security zone
   - **IPv4/IPv6** - Set static addresses
   - **DHCP Client** - Enable to get IP from upstream
   - **MTU** - Set maximum transmission unit
4. Click **Save**

Changes are **staged** (not applied immediately). You'll see a pending changes bar at the top.

#### Creating VLANs

1. Click **+ Add VLAN** on a parent interface
2. Enter the VLAN ID (1-4094)
3. Configure IP addresses and zone
4. Click **Create**

#### Creating Bonds

1. Click **+ Create Bond** in the header
2. Select member interfaces
3. Choose bonding mode:
   - `802.3ad` (LACP) - Requires switch support
   - `balance-rr` - Round-robin
   - `active-backup` - Failover
4. Configure bond IP and zone
5. Click **Create**

### Uplinks

The **Uplinks** page configures WAN connections for multi-WAN setups.

- **Priority** - Lower numbers are preferred
- **Weight** - For load balancing ratios
- **Health Checks** - Configure ping targets and thresholds

---

## Policy

The **Policy** page manages firewall rules between zones.

### Policy Matrix

The matrix shows allowed/denied traffic between zone pairs:
- **Green** - Traffic allowed
- **Red** - Traffic blocked
- **Click** any cell to view/edit rules

### Adding a Rule

1. Click on a zone pair (e.g., LAN → WAN)
2. Click **+ Add Rule**
3. Configure:
   - **Name** - Descriptive rule name
   - **Action** - Accept, Drop, or Reject
   - **Protocol** - TCP, UDP, ICMP, or Any
   - **Ports** - Destination port(s)
   - **Source/Destination** - IP addresses or IPSets
4. Click **Save**

### Rule Order

Rules are evaluated top-to-bottom. Drag rules to reorder priority.

---

## DNS

The **DNS** page manages the built-in DNS resolver.

### Overview

Shows DNS statistics:
- **Queries** - Total DNS queries handled
- **Cache Hit Rate** - Percentage served from cache
- **Blocked Queries** - Queries matched by blocklists

### Blocklists

Manage ad and malware blocklists:

1. Click **+ Add Blocklist**
2. Enter a URL (e.g., `https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts`)
3. Choose format: `hosts`, `domains`, or `adblock`
4. Click **Save**

Blocklists are automatically refreshed daily.

### Upstream Servers

Configure where Flywall forwards DNS queries:

1. Click **Upstreams** tab
2. Add servers with their type:
   - **Plain** - Traditional UDP/TCP DNS
   - **DoH** - DNS over HTTPS
   - **DoT** - DNS over TLS

### Query Log

View recent DNS queries for troubleshooting:
- Filter by domain, client, or response type
- See which blocklist blocked a query

---

## Tunnels (VPN)

The **Tunnels** page manages VPN connections.

### WireGuard

View and manage WireGuard interfaces:

| Column | Description |
|--------|-------------|
| **Name** | Interface name (e.g., `wg0`) |
| **Status** | Connected/disconnected |
| **Peers** | Number of configured peers |
| **Transfer** | Data transferred |

#### Adding a Peer

1. Click on a WireGuard interface
2. Click **+ Add Peer**
3. Enter:
   - **Name** - Friendly peer name
   - **Public Key** - Peer's WireGuard public key
   - **Allowed IPs** - Networks to route through this peer
   - **Endpoint** - (Optional) Peer's public IP:port
4. Click **Save**

#### Generating Client Config

1. Click on a peer
2. Click **Show Config**
3. Scan the QR code with the WireGuard mobile app, or copy the text config

### Tailscale

If connected to Tailscale:
- View network status
- See connected devices
- Manage advertised routes

---

## Discovery

The **Discovery** page provides network visibility and learning features.

### Device Discovery

See all devices on your network:
- **MAC Address** and vendor
- **IP Address** (DHCP or static)
- **Hostname** (from DHCP or mDNS)
- **First Seen / Last Seen**

### Flow Tracking

When learning mode is enabled:
- New connections are logged
- Pending rules can be approved or denied
- Build firewall rules based on actual traffic

---

## System

### Settings

General system configuration:

| Setting | Description |
|---------|-------------|
| **Hostname** | System hostname |
| **Timezone** | System timezone |
| **Theme** | Light/Dark/Auto |
| **Log Retention** | How long to keep logs |

### Users

Manage UI accounts:

1. Click **+ Add User**
2. Enter username and password
3. Assign role:
   - **Admin** - Full access
   - **Operator** - View + apply, no user management
   - **Viewer** - Read-only access
4. Click **Create**

### Groups

Create groups for shared permissions (coming soon).

### Alerts

Configure notifications:
- **Email** - SMTP settings for email alerts
- **Webhook** - HTTP endpoints for automation

---

## Staged Configuration

Flywall uses a **staged configuration model** for safety.

### How It Works

1. Make changes (interfaces, policies, DNS, etc.)
2. Changes appear in the **Pending Changes** bar
3. Click **Review** to see a diff
4. Click **Apply** to activate changes
5. Or click **Discard** to revert

### Why Staging?

- **Preview before applying** - See exactly what will change
- **Atomic updates** - All changes apply together or not at all
- **Rollback safety** - If something breaks, discard and revert

### The Pending Changes Bar

When you have staged changes, a bar appears at the top:

```
┌──────────────────────────────────────────────────────────┐
│ 📝 3 pending changes                    [Review] [Apply] │
└──────────────────────────────────────────────────────────┘
```

- Click **Review** to see a diff of all changes
- Click **Apply** to commit changes to the running system
- Click **Discard** to throw away all pending changes

---

## Real-Time Updates

The UI uses WebSockets for live updates:
- **Interface status** changes instantly when links go up/down
- **DHCP leases** appear as devices connect
- **Traffic stats** update in real-time
- **Alerts** pop up immediately

No need to refresh the page!

---

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `?` | Show keyboard shortcuts |
| `g` `d` | Go to Dashboard |
| `g` `n` | Go to Network |
| `g` `p` | Go to Policy |
| `g` `s` | Go to System |
| `Esc` | Close dialogs/modals |

---

## Mobile Access

The Web UI is fully responsive:
- Sidebar collapses to a hamburger menu
- Touch-friendly controls
- Works on phones and tablets

---

## Troubleshooting

### Can't Access Web UI

1. Check you're connecting from an allowed zone (usually LAN)
2. Verify the web service is running:
   ```bash
   ss -tlnp | grep 8080
   ```
3. Check zone management settings allow `web_ui`

### Changes Not Applying

1. Make sure you clicked **Apply** (not just Save)
2. Check the pending changes bar for errors
3. Review the diff for validation issues

### Login Issues

1. Clear browser cookies and try again
2. Reset password via CLI if locked out:
   ```bash
   flywall user reset-password admin
   ```
