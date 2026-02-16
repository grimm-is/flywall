---
title: "API Reference"
linkTitle: "API"
weight: 20
description: >
  REST API for programmatic access.
---

Flywall provides a RESTful API for configuration and monitoring.

## Base URL

```
http://localhost:8080/api  (HTTP)
https://localhost:8443/api (HTTPS)
```

## Authentication

### API Keys

Include API key in the `Authorization` header:

```bash
curl -H "Authorization: Bearer fw_your_api_key_here" \
  http://localhost:8080/api/status
```

### Session Cookies

Web UI uses session-based authentication. After login, cookies are included automatically.

## Endpoints

### Status

```http
GET /api/status
```

Returns system status including uptime, version, and service health.

### Configuration

```http
GET /api/config
GET /api/config?source=staged   # Staged config
GET /api/config?source=running  # Running config
POST /api/config                # Update (full replace)
PATCH /api/config               # Partial update
```

### Staged Configuration

```http
GET /api/config/diff            # Diff between staged and running
POST /api/config/apply          # Apply staged to running
POST /api/config/discard        # Discard staged changes
```

### Interfaces

```http
GET /api/interfaces
GET /api/interfaces/{name}
PUT /api/interfaces/{name}
```

### DHCP

```http
GET /api/dhcp/leases
DELETE /api/dhcp/leases/{ip}
GET /api/dhcp/scopes
```

### DNS

```http
GET /api/dns/stats
POST /api/dns/flush
GET /api/dns/query?name={domain}
```

### VPN

```http
GET /api/vpn/wireguard
GET /api/vpn/wireguard/{interface}/peers
```

### Firewall

```http
GET /api/firewall/rules
GET /api/firewall/connections   # Active connections
```

## WebSocket Events

Connect to `/api/ws` for real-time events:

```javascript
const ws = new WebSocket('ws://localhost:8080/api/ws');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.type, data.payload);
};
```

Event types:
- `dhcp.lease.new` - New DHCP lease
- `dhcp.lease.expired` - Lease expired
- `interface.state` - Interface up/down
- `config.changed` - Configuration updated
- `vpn.peer.connected` - VPN peer connected

## OpenAPI Specification

Interactive API documentation is available at:

```
http://localhost:8080/api/swagger/
```

Or download the OpenAPI spec:

```
http://localhost:8080/api/openapi.json
```

## Examples

### Get All Interfaces

```bash
curl -s http://localhost:8080/api/interfaces | jq
```

### Update Interface

```bash
curl -X PUT http://localhost:8080/api/interfaces/eth1 \
  -H "Content-Type: application/json" \
  -d '{"ipv4": ["192.168.1.1/24"], "zone": "LAN"}'
```

### Apply Staged Config

```bash
curl -X POST http://localhost:8080/api/config/apply
```

## Error Handling

Errors return appropriate HTTP status codes with JSON body:

```json
{
  "error": "validation_error",
  "message": "Invalid IP address format",
  "field": "ipv4"
}
```

| Status | Meaning |
|--------|---------|
| 400 | Bad Request (validation error) |
| 401 | Unauthorized (missing/invalid auth) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not Found |
| 409 | Conflict (e.g., resource in use) |
| 500 | Internal Server Error |
