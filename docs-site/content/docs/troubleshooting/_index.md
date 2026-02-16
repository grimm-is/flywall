---
title: "Troubleshooting"
linkTitle: "Troubleshooting"
weight: 60
description: >
  Common issues and how to resolve them.
---

## Diagnostic Tools

### Check Service Status

```bash
# Systemd status
sudo systemctl status flywall

# Flywall built-in status
flywall status
```

### View Logs

```bash
# Systemd journal
journalctl -u flywall -f

# Flywall log files
tail -f /var/log/flywall/flywall.log
```

### Validate Configuration

```bash
flywall validate -c /etc/flywall/flywall.hcl
```

---

## Common Issues

### Flywall Won't Start

**Symptom:** `flywall start` exits immediately or fails.

**Check:**
1. Configuration syntax:
   ```bash
   flywall validate
   ```

2. Another instance running:
   ```bash
   pgrep flywall
   ```

3. Port conflicts:
   ```bash
   ss -tlnp | grep -E ':(53|67|8080)'
   ```

4. Permissions:
   ```bash
   ls -la /etc/flywall/
   ls -la /var/lib/flywall/
   ```

---

### No Internet Access from LAN

**Symptom:** Devices get DHCP but can't reach the internet.

**Check:**
1. IP forwarding enabled:
   ```bash
   cat /proc/sys/net/ipv4/ip_forward
   # Should be 1
   ```

2. NAT rules exist:
   ```bash
   sudo nft list chain inet fw nat_postrouting
   ```

3. WAN has IP and default route:
   ```bash
   ip addr show eth0
   ip route show default
   ```

4. WAN can reach internet:
   ```bash
   ping -I eth0 8.8.8.8
   ```

**Fix:** Ensure your config has:
```hcl
ip_forwarding = true

nat "outbound" {
  type          = "masquerade"
  out_interface = "eth0"
}
```

---

### DHCP Not Working

**Symptom:** Clients don't get IP addresses.

**Check:**
1. DHCP listening:
   ```bash
   ss -ulnp | grep :67
   ```

2. Interface has correct IP:
   ```bash
   ip addr show eth1
   ```

3. DHCP range is valid:
   - Range must be in same subnet as interface IP
   - Range must not overlap with static IPs

4. Firewall not blocking:
   ```bash
   sudo nft list ruleset | grep -i dhcp
   ```

**Debug:**
```bash
# Watch DHCP requests
flywall dhcp debug
```

---

### DNS Not Resolving

**Symptom:** Can ping IPs but not hostnames.

**Check:**
1. DNS listening:
   ```bash
   ss -ulnp | grep :53
   ```

2. Direct query works:
   ```bash
   dig @localhost google.com
   ```

3. Upstream reachable:
   ```bash
   flywall dns test-upstream
   ```

4. Check blocklists aren't over-blocking:
   ```bash
   flywall dns lookup domain-that-fails.com
   ```

---

### Web UI Not Accessible

**Symptom:** Can't reach http://192.168.1.1:8080

**Check:**
1. Web server listening:
   ```bash
   ss -tlnp | grep :8080
   ```

2. Zone allows management:
   ```hcl
   zone "LAN" {
     management {
       web_ui = true
     }
   }
   ```

3. Connecting from allowed zone:
   - Web UI only accessible from zones with `web_ui = true`

4. Firewall rules:
   ```bash
   sudo nft list ruleset | grep 8080
   ```

---

### Port Forward Not Working

**Symptom:** External access to port forward fails.

**Check:**
1. NAT rule exists:
   ```bash
   sudo nft list chain inet fw nat_prerouting | grep dnat
   ```

2. Policy allows traffic:
   ```bash
   flywall debug trace --proto tcp --dport <port>
   ```

3. Internal server listening:
   ```bash
   nc -zv 192.168.1.x <internal_port>
   ```

4. ISP not blocking port:
   - Try from mobile hotspot (different ISP)

---

### VPN Peers Not Connecting

**Symptom:** WireGuard handshake fails.

**Check:**
1. WireGuard listening:
   ```bash
   ss -ulnp | grep 51820
   ```

2. Firewall allows UDP 51820:
   ```bash
   sudo nft list ruleset | grep 51820
   ```

3. Keys correct:
   - Server public key in client config
   - Client public key in server config

4. Endpoint reachable:
   ```bash
   nc -uzv your.domain.com 51820
   ```

---

### Slow Performance

**Symptom:** Network slower than expected.

**Check:**
1. CPU usage:
   ```bash
   top -p $(pgrep flywall)
   ```

2. Interface errors:
   ```bash
   ip -s link show
   ```

3. Connection tracking table:
   ```bash
   cat /proc/sys/net/netfilter/nf_conntrack_count
   cat /proc/sys/net/netfilter/nf_conntrack_max
   ```

4. Firewall rule count:
   ```bash
   sudo nft list ruleset | wc -l
   ```

**Fix:**
- Increase conntrack max if near limit
- Simplify rules if count is very high
- Check for hardware offload support

---

## Debug Mode

Run Flywall with verbose debugging:

```bash
flywall start --foreground --debug
```

Or set log level via environment:

```bash
FLYWALL_LOG_LEVEL=debug flywall start
```

---

## Collecting Diagnostics

When reporting issues, collect:

```bash
# System info
uname -a
flywall version

# Configuration (sanitize secrets!)
flywall debug config

# Running state
flywall status --json

# Active rules
sudo nft list ruleset > rules.txt

# Recent logs
journalctl -u flywall --since "1 hour ago" > logs.txt
```

---

## Getting Help

- **Documentation**: https://docs.flywall.dev
- **GitHub Issues**: https://github.com/grimm-is/flywall/issues
- **Discussions**: https://github.com/grimm-is/flywall/discussions
