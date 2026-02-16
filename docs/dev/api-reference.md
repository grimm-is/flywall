# API Reference Implementation Guide

## Overview

Flywall provides a comprehensive REST API for:
- Configuration management
- Status monitoring
- Real-time statistics
- Log access
- Feature control

## Architecture

### API Components
1. **HTTP Server**: RESTful API endpoints
2. **Authentication**: Token-based auth (optional)
3. **Middleware**: Request logging, CORS, rate limiting
4. **Handlers**: Feature-specific endpoint handlers
5. **WebSocket**: Real-time updates

### API Versions
- Current version: v1
- Versioning via URL path: `/api/v1/`
- Backward compatibility maintained

## Configuration

### Basic API Setup
```hcl
api {
  enabled = true
  listen = "0.0.0.0:8080"

  # Authentication
  require_auth = true
  auth_token = "YOUR_API_TOKEN"

  # CORS
  cors = {
    allowed_origins = ["*"]
    allowed_methods = ["GET", "POST", "PUT", "DELETE"]
    allowed_headers = ["Content-Type", "Authorization"]
  }
}
```

### Advanced API Configuration
```hcl
api {
  enabled = true
  listen = "0.0.0.0:8080"
  tls {
    enabled = true
    cert_file = "/etc/flywall/cert.pem"
    key_file = "/etc/flywall/key.pem"
  }

  # Authentication
  require_auth = true
  auth_token = "SECURE_TOKEN_32_CHARS"
  auth_timeout = "1h"

  # Rate limiting
  rate_limit = {
    enabled = true
    requests_per_minute = 60
    burst = 10
  }

  # Logging
  access_log = true
  log_level = "info"

  # WebSocket
  websocket = {
    enabled = true
    path = "/ws"
    ping_interval = "30s"
  }

  # CORS
  cors = {
    allowed_origins = ["https://ui.example.com"]
    allowed_methods = ["GET", "POST", "PUT", "DELETE", "PATCH"]
    allowed_headers = ["Content-Type", "Authorization", "X-Requested-With"]
    exposed_headers = ["X-Total-Count"]
    max_age = "86400"
  }
}
```

## API Endpoints

### System Information
```bash
# System status
GET /api/status

# System information
GET /api/system/info

# System resources
GET /api/system/resources

# Version information
GET /api/version
```

### Configuration Management
```bash
# Get current configuration
GET /api/config

# Get configuration section
GET /api/config/zones

# Update configuration
PUT /api/config
Content-Type: application/json
{
  "schema_version": "1.0",
  "zones": {...}
}

# Validate configuration
POST /api/config/validate
Content-Type: application/json
{
  "config": {...}
}

# Apply staged configuration
POST /api/config/apply

# Get configuration diff
GET /api/config/diff
```

### Zone Management
```bash
# List all zones
GET /api/zones

# Get specific zone
GET /api/zones/{zone_name}

# Create zone
POST /api/zones
Content-Type: application/json
{
  "name": "DMZ",
  "interfaces": ["eth2"],
  "description": "Demilitarized Zone"
}

# Update zone
PUT /api/zones/{zone_name}

# Delete zone
DELETE /api/zones/{zone_name}
```

### Interface Management
```bash
# List interfaces
GET /api/interfaces

# Get interface details
GET /api/interfaces/{interface_name}

# Update interface
PUT /api/interfaces/{interface_name}
Content-Type: application/json
{
  "ipv4": ["192.168.1.1/24"],
  "zone": "lan"
}
```

### Policy Management
```bash
# List policies
GET /api/policies

# Get policy
GET /api/policies/{policy_id}

# Create policy
POST /api/policies
Content-Type: application/json
{
  "src_zone": "Green",
  "dst_zone": "WAN",
  "rules": [...]
}

# Update policy
PUT /api/policies/{policy_id}

# Delete policy
DELETE /api/policies/{policy_id}
```

### NAT Rules
```bash
# List NAT rules
GET /api/nat

# Get NAT rule
GET /api/nat/{rule_name}

# Create NAT rule
POST /api/nat
Content-Type: application/json
{
  "name": "masq",
  "type": "masquerade",
  "out_interface": "eth0"
}
```

### DHCP Management
```bash
# Get DHCP status
GET /api/dhcp/status

# List scopes
GET /api/dhcp/scopes

# Get scope details
GET /api/dhcp/scopes/{scope_name}

# List leases
GET /api/dhcp/leases

# Get specific lease
GET /api/dhcp/leases/{mac_address}

# Add reservation
POST /api/dhcp/reservations
Content-Type: application/json
{
  "name": "server1",
  "mac": "00:11:22:33:44:55",
  "ip": "192.168.1.10",
  "scope": "lan"
}
```

### DNS Management
```bash
# Get DNS status
GET /api/dns/status

# Query DNS
GET /api/dns/query?name=example.com&type=A

# Get DNS cache
GET /api/dns/cache

# Clear DNS cache
DELETE /api/dns/cache

# Get query log
GET /api/dns/queries?limit=100
```

### Learning Engine
```bash
# Get learning status
GET /api/learning/status

# List flows
GET /api/learning/flows

# Get suggestions
GET /api/learning/suggestions

# Apply suggestion
POST /api/learning/suggestions/{id}/apply

# Get learning stats
GET /api/learning/stats
```

### VPN Management
```bash
# Get VPN status
GET /api/vpn/status

# Get WireGuard config
GET /api/vpn/wireguard/config

# List peers
GET /api/vpn/wireguard/peers

# Add peer
POST /api/vpn/wireguard/peers
Content-Type: application/json
{
  "name": "client1",
  "public_key": "...",
  "allowed_ips": ["10.200.0.10/32"]
}

# Export client config
GET /api/vpn/wireguard/export/{peer_name}
```

### High Availability
```bash
# Get HA status
GET /api/ha/status

# Get cluster info
GET /api/ha/cluster

# Force failover
POST /api/ha/failover

# Get replication status
GET /api/replication/status
```

### Monitoring & Metrics
```bash
# Get metrics
GET /api/metrics

# Get alerts
GET /api/alerts

# Get analytics data
GET /api/analytics/traffic
GET /api/analytics/top-talkers

# Get logs
GET /api/logs?limit=100&level=error
```

## WebSocket API

### Real-time Updates
```javascript
// Connect to WebSocket
const ws = new WebSocket('ws://localhost:8080/ws');

// Subscribe to events
ws.send(JSON.stringify({
  action: 'subscribe',
  events: ['stats', 'alerts', 'logs']
}));

// Receive updates
ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('Update:', data);
};
```

### Event Types
- `stats`: Real-time statistics
- `alerts`: Alert notifications
- `logs`: Log stream
- `config`: Configuration changes
- `ha`: HA status changes

## Authentication

### Token-based Auth
```bash
# Include token in header
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://firewall.example.com/api/status

# Or as query parameter
curl https://firewall.example.com/api/status?token=YOUR_TOKEN
```

### Session Management
```bash
# Create session
POST /api/auth/login
Content-Type: application/json
{
  "username": "admin",
  "password": "password"
}

# Refresh token
POST /api/auth/refresh
Authorization: Bearer REFRESH_TOKEN

# Logout
POST /api/auth/logout
```

## Testing

### Integration Tests
- `api_test.sh`: Basic API connectivity
- `api_auth_test.sh`: Authentication
- `api_crud_test.sh`: CRUD operations
- `api_websocket_test.sh`: WebSocket functionality

### Manual Testing
```bash
# Test API with curl
curl -s http://localhost:8080/api/status | jq .

# Test POST with JSON
curl -X POST http://localhost:8080/api/config/validate \
  -H "Content-Type: application/json" \
  -d @config.json

# Test WebSocket
wscat -c ws://localhost:8080/ws
```

## Best Practices

1. **Security**
   - Always use authentication in production
   - Enable TLS for external access
   - Use rate limiting
   - Audit API access

2. **Performance**
   - Use pagination for large datasets
   - Cache frequently accessed data
   - Compress responses
   - Monitor API response times

3. **Error Handling**
   - Check HTTP status codes
   - Parse error responses
   - Implement retry logic
   - Log failed requests

4. **Versioning**
   - Use API version in URL
   - Maintain backward compatibility
   - Document deprecations
   - Use semantic versioning

## Troubleshooting

### Common Issues
1. **401 Unauthorized**: Check auth token
2. **404 Not Found**: Verify endpoint path
3. **500 Internal Error**: Check server logs

### Debug Commands
```bash
# Check API logs
journalctl -u flywall | grep api

# Test with verbose curl
curl -v http://localhost:8080/api/status

# Check TLS certificate
openssl s_client -connect firewall.example.com:443

# Monitor API traffic
tcpdump -i any port 8080 -A
```

## SDK Examples

### Go
```go
import "github.com/flywall/client"

client := client.New("http://localhost:8080", "token")
status, err := client.GetStatus()
```

### Python
```python
import flywall_api

client = flywall_api.Client("http://localhost:8080", "token")
status = client.get_status()
```

### JavaScript
```javascript
import { FlywallClient } from 'flywall-api';

const client = new FlywallClient('http://localhost:8080', 'token');
const status = await client.getStatus();
```

## Related Features

- [Configuration Management](config-management.md)
- [Metrics Collection](metrics-collection.md)
- [Alerting System](alerting.md)
- [CLI Tools](cli-tools.md)
