# WireGuard VPN Implementation Guide

## Overview

Flywall includes full WireGuard VPN support for:
- Site-to-site VPN connections
- Remote client VPN access
- VPN client import/export
- VPN isolation and routing policies
- VPN lockout protection

## Architecture

### WireGuard Integration
- Native WireGuard kernel module integration
- Automatic interface creation and configuration
- Peer management through HCL configuration
- Handshake monitoring and statistics
- Key management and rotation

### VPN Zones
- Dedicated VPN zone for traffic segregation
- Policy-based routing for VPN traffic
- Isolation between VPN and other zones
- Fail-open support for VPN connectivity

## Configuration

### Basic Site-to-Site VPN
```hcl
# WireGuard VPN configuration
wireguard "wg0" {
  enabled = true
  interface = "wg0"
  private_key = "PRIVATE_KEY_HERE"
  listen_port = 51820

  peer "remote-site" {
    public_key = "REMOTE_PUBLIC_KEY"
    allowed_ips = ["192.168.100.0/24", "10.200.0.2/32"]
    endpoint = "203.0.113.10:51820"
    persistent_keepalive = 25
  }
}

# VPN interface configuration
interface "wg0" {
  ipv4 = ["10.200.0.1/24"]
  zone = "vpn"
}

# VPN zone definition
zone "vpn" {
  interface = "wg0"
}
```

### Remote Access VPN
```hcl
wireguard "wg0" {
  enabled = true
  interface = "wg0"
  private_key = "SERVER_PRIVATE_KEY"
  listen_port = 51820

  # Client template
  peer "client1" {
    public_key = "CLIENT1_PUBLIC_KEY"
    allowed_ips = ["10.200.0.10/32"]
  }

  peer "client2" {
    public_key = "CLIENT2_PUBLIC_KEY"
    allowed_ips = ["10.200.0.11/32"]
  }
}

# VPN interface with client pool
interface "wg0" {
  ipv4 = ["10.200.0.1/24"]
  zone = "vpn"
}
```

### VPN with Multiple Peers
```hcl
wireguard "wg0" {
  enabled = true
  interface = "wg0"
  private_key = "PRIVATE_KEY"
  listen_port = 51820

  # Site A
  peer "site-a" {
    public_key = "SITE_A_PUBLIC_KEY"
    allowed_ips = ["10.1.0.0/24"]
    endpoint = "site-a.example.com:51820"
  }

  # Site B
  peer "site-b" {
    public_key = "SITE_B_PUBLIC_KEY"
    allowed_ips = ["10.2.0.0/24"]
    endpoint = "site-b.example.com:51820"
  }

  # Road warrior client
  peer "mobile" {
    public_key = "MOBILE_PUBLIC_KEY"
    allowed_ips = ["10.200.0.100/32"]
    # No endpoint - client initiates
  }
}
```

### VPN Isolation
```hcl
# VPN zone - restricted access
zone "vpn" {
  interface = "wg0"
}

# Allow VPN to access internal services
policy "vpn" "Green" {
  name = "vpn_to_internal"

  rule "allow_specific" {
    description = "Allow VPN to specific internal services"
    proto     = "tcp"
    dest_port = [22, 443, 8080]
    action    = "accept"
  }

  rule "block_all" {
    description = "Block other access"
    action = "drop"
    log = true
  }
}

# Allow internal to VPN
policy "Green" "vpn" {
  name = "internal_to_vpn"

  rule "allow_all" {
    description = "Allow internal to initiate VPN connections"
    action = "accept"
  }
}
```

### VPN Lockout Protection
```hcl
wireguard "wg0" {
  enabled = true
  interface = "wg0"
  private_key = "PRIVATE_KEY"
  listen_port = 51820

  # Enable lockout protection
  lockout_protection = true
  max_failures = 5
  lockout_duration = "5m"

  peer "client1" {
    public_key = "CLIENT1_PUBLIC_KEY"
    allowed_ips = ["10.200.0.10/32"]
  }
}
```

## Implementation Details

### Key Management
```hcl
# Generate keys via API
curl -X POST "http://localhost:8080/api/vpn/wireguard/keys"

# Or via CLI
flywall vpn wireguard genkey > private.key
wg pubkey < private.key > public.key
```

### Handshake Monitoring
- Automatic handshake detection
- Peer status tracking
- Connection statistics
- Keepalive monitoring

### VPN Routing
- Routes automatically added for allowed_ips
- Policy-based routing for complex scenarios
- NAT traversal support
- MTU handling

## Testing

### Integration Tests
- `vpn_test.sh`: Basic site-to-site VPN
- `vpn_isolation_test.sh`: Traffic isolation
- `vpn_lockout_test.sh`: Lockout protection
- `wireguard-client-import.sh`: Client configuration
- `tailscale_*`: Tailscale integration tests

### Manual Testing
```bash
# Check WireGuard status
wg show

# Check specific interface
wg show wg0

# Monitor handshakes
watch wg show wg0 latest-handshakes

# Test connectivity
ping 10.200.0.2

# Check routing
ip route show table 100
```

## API Integration

### VPN Management API
```bash
# Get VPN status
curl -s "http://localhost:8080/api/vpn/status"

# Get WireGuard configuration
curl -s "http://localhost:8080/api/vpn/wireguard/config"

# Get peer list
curl -s "http://localhost:8080/api/vpn/wireguard/peers"

# Add peer
curl -X POST "http://localhost:8080/api/vpn/wireguard/peers" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "new-peer",
    "public_key": "PUBLIC_KEY",
    "allowed_ips": ["10.200.0.50/32"]
  }'

# Remove peer
curl -X DELETE "http://localhost:8080/api/vpn/wireguard/peers/peer-name"
```

### Client Configuration Export
```bash
# Export client config
curl -s "http://localhost:8080/api/vpn/wireguard/export/client1" > client1.conf

# Export QR code for mobile apps
curl -s "http://localhost:8080/api/vpn/wireguard/qr/client1"
```

## Best Practices

1. **Security**
   - Use strong private keys
   - Regularly rotate keys
   - Limit allowed_ips to necessary ranges
   - Enable lockout protection

2. **Performance**
   - Use persistent keepalives for NAT traversal
   - Monitor handshake success rates
   - Consider MTU for VPN overhead

3. **Reliability**
   - Configure backup endpoints
   - Monitor peer connectivity
   - Use appropriate keepalive intervals

4. **Network Design**
   - Plan IP addressing carefully
   - Document VPN topology
   - Test failover scenarios

## Troubleshooting

### Common Issues
1. **No handshake**: Check firewall rules and endpoints
2. **One-way traffic**: Verify allowed_ips and routing
3. **Connection drops**: Check keepalive settings

### Debug Commands
```bash
# Check WireGuard logs
journalctl -u flywall | grep wireguard

# Monitor packets
tcpdump -i wg0

# Check interface status
ip addr show wg0

# Verify routing
ip route get 10.200.0.2

# Test with verbose
wg show wg0 dump
```

### Advanced Debugging
```bash
# Manual peer configuration
wg set wg0 peer PUBLIC_KEY allowed-ips 10.200.0.2/32 endpoint HOST:PORT

# Remove peer
wg set wg0 peer PUBLIC_KEY remove

# Restart interface
wg-quick down wg0
wg-quick up wg0
```

## Performance Considerations

- WireGuard uses kernel-space processing for high performance
- CPU usage minimal for established connections
- Handshake CPU-intensive but infrequent
- Consider hardware acceleration for high-throughput scenarios

## Security Considerations

- All traffic encrypted with ChaCha20-Poly1305
- Perfect forward secrecy
- No fixed ports - can use any UDP port
- Minimal attack surface
- Regular security audits recommended

## Related Features

- [VPN Isolation](vpn-isolation.md)
- [Tailscale Integration](tailscale.md)
- [Zone Policies](zones-policies.md)
- [NAT & Routing](nat-routing.md)
