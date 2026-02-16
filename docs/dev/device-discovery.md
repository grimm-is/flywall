# Device Discovery Implementation Guide

## Overview

Flywall provides device discovery for:
- Network device identification
- Device fingerprinting
- Vendor detection
- Device classification
- Asset inventory

## Architecture

### Discovery Components
1. **Discovery Engine**: Coordinates discovery processes
2. **Scanner**: Active network scanning
3. **Passive Listener**: Passive traffic analysis
4. **Fingerprinter**: Device fingerprinting
5. **Database**: Device information storage

### Discovery Methods
- **Active Scanning**: Direct probing
- **Passive Listening**: Traffic analysis
- **ARP Monitoring**: ARP table monitoring
- **DHCP Leases**: DHCP lease analysis
- **SNMP Queries**: SNMP device queries

## Configuration

### Basic Device Discovery Setup
```hcl
# Device discovery configuration
device_discovery {
  enabled = true

  # Scan settings
  scan_interval = "1h"
  networks = ["192.168.1.0/24", "192.168.2.0/24"]
}
```

### Advanced Device Discovery Configuration
```hcl
device_discovery {
  enabled = true

  # Discovery methods
  methods = {
    # Active scanning
    active = {
      enabled = true

      # Scan settings
      scan_interval = "1h"
      scan_timeout = "5s"
      max_concurrent = 50

      # Networks to scan
      networks = [
        "192.168.1.0/24",
        "192.168.2.0/24",
        "10.0.0.0/8"
      ]

      # Ports to scan
      ports = [
        22,    # SSH
        23,    # Telnet
        53,    # DNS
        80,    # HTTP
        135,   # RPC
        139,   # NetBIOS
        443,   # HTTPS
        445,   # SMB
        993,   # IMAPS
        995,   # POP3S
        1433,  # SQL Server
        3389,  # RDP
        5432,  # PostgreSQL
        6379,  # Redis
        8080,  # HTTP Alt
        8443   # HTTPS Alt
      ]

      # Scan techniques
      techniques = ["tcp_connect", "tcp_syn", "udp", "icmp"]

      # Rate limiting
      rate_limit = {
        packets_per_second = 100
        burst = 200
      }
    }

    # Passive discovery
    passive = {
      enabled = true

      # Interfaces to listen on
      interfaces = ["eth0", "eth1", "eth2"]

      # What to collect
      collect = {
        arp = true
        dhcp = true
        dns = true
        http = true
        tls = true
        smb = true
      }

      # Flow analysis
      flow_analysis = {
        enabled = true
        timeout = "5m"
        min_packets = 10
      }
    }

    # ARP monitoring
    arp_monitor = {
      enabled = true

      # Update interval
      update_interval = "30s"

      # Aging
      aging_time = "1h"

      # Static entries
      static = {
        "192.168.1.1" = "aa:bb:cc:dd:ee:ff"
      }
    }

    # DHCP lease analysis
    dhcp_analysis = {
      enabled = true

      # DHCP servers to monitor
      servers = ["192.168.1.1"]

      # Lease database
      lease_database = "/var/lib/dhcp/dhcpd.leases"
    }
  }

  # Fingerprinting
  fingerprinting = {
    enabled = true

    # OS fingerprinting
    os_fingerprinting = {
      enabled = true
      methods = ["ttl", "window_size", "options", "banner"]

      # Signatures
      signature_database = "/etc/flywall/os-signatures.db"
    }

    # Service fingerprinting
    service_fingerprinting = {
      enabled = true

      # Service signatures
      signatures = [
        {
          name = "ssh"
          pattern = "SSH-.*"
          port = 22
        },
        {
          name = "http"
          pattern = "Server: .*"
          port = 80
        },
        {
          name = "https"
          pattern = "TLS.*"
          port = 443
        }
      ]
    }

    # Device fingerprinting
    device_fingerprinting = {
      enabled = true

      # MAC OUI database
      oui_database = "/etc/flywall/oui-db.txt"

      # Custom fingerprints
      custom = [
        {
          vendor = "Cisco"
          pattern = "cisco.*"
          confidence = 0.9
        },
        {
          vendor = "Juniper"
          pattern = "juniper.*"
          confidence = 0.9
        }
      ]
    }
  }

  # Classification
  classification = {
    enabled = true

    # Device types
    types = [
      {
        name = "server"
        criteria = {
          open_ports = [22, 80, 443, 3389]
          os_patterns = ["Linux", "Windows Server"]
          services = ["ssh", "http", "https"]
        }
      },
      {
        name = "workstation"
        criteria = {
          open_ports = [22, 135, 139, 445, 3389]
          os_patterns = ["Windows", "macOS", "Linux Desktop"]
        }
      },
      {
        name = "network_device"
        criteria = {
          mac_vendor = ["Cisco", "Juniper", "Arista", "Ubiquiti"]
          open_ports = [22, 23, 80, 443]
        }
      },
      {
        name = "iot_device"
        criteria = {
          open_ports = [80, 443, 8080, 8443]
          mac_vendor = ["Google", "Amazon", "Apple", "Samsung"]
          http_patterns = ["IoT", "Smart"]
        }
      },
      {
        name = "printer"
        criteria = {
          open_ports = [80, 443, 515, 631, 9100]
          snmp = true
          services = ["http", "ipp", "lpd"]
        }
      }
    ]
  }

  # Database
  database = {
    type = "sqlite"
    path = "/var/lib/flywall/device_discovery.db"

    # Retention
    retention = {
      seen_devices = "30d"
      offline_devices = "7d"
      history = "90d"
    }
  }
}
```

### Scheduled Discovery
```hcl
device_discovery {
  enabled = true

  # Scheduled scans
  schedules = [
    {
      name = "full_scan"
      description = "Full network scan"

      # Schedule (cron format)
      schedule = "0 2 * * *"  # Daily at 2 AM

      # Networks
      networks = ["192.168.0.0/16"]

      # Scan type
      scan_type = "full"

      # Ports
      ports = "common"

      # Actions
      actions = ["scan", "fingerprint", "classify", "report"]
    },
    {
      name = "quick_scan"
      description = "Quick discovery scan"

      schedule = "*/30 * * * *"  # Every 30 minutes

      networks = ["192.168.1.0/24"]

      scan_type = "quick"

      ports = [22, 80, 443, 3389]

      actions = ["scan", "update"]
    },
    {
      name = "iot_scan"
      description = "IoT device scan"

      schedule = "0 */6 * * *"  # Every 6 hours

      networks = ["192.168.200.0/24"]

      scan_type = "iot"

      ports = [80, 443, 8080, 8443, 1883, 5683]

      actions = ["scan", "fingerprint", "classify"]
    }
  ]
}
```

### Integration with Other Features
```hcl
device_discovery {
  enabled = true

  # Integration with zones
  zone_integration = {
    enabled = true

    # Auto-assign devices to zones
    auto_assign = true

    # Zone mapping
    zone_mapping = [
      {
        network = "192.168.1.0/24"
        zone = "LAN"
      },
      {
        network = "192.168.200.0/24"
        zone = "Guest"
      },
      {
        network = "10.200.0.0/24"
        zone = "VPN"
      }
    ]
  }

  # Integration with policies
  policy_integration = {
    enabled = true

    # Auto-generate policies
    auto_policies = true

    # Policy templates
    templates = [
      {
        device_type = "iot_device"
        policy = "iot_restrictive"
      },
      {
        device_type = "printer"
        policy = "printer_access"
      },
      {
        device_type = "unknown"
        policy = "quarantine"
      }
    ]
  }

  # Integration with monitoring
  monitoring_integration = {
    enabled = true

    # Monitor device changes
    monitor_changes = true

    # Alert on new devices
    alert_new_devices = true

    # Alert on device changes
    alert_changes = true

    # Alert thresholds
    thresholds = {
      new_devices_per_hour = 10
      changes_per_hour = 50
    }
  }
}
```

## Implementation Details

### Discovery Process
1. Network enumeration
2. Port scanning
3. Service detection
4. OS fingerprinting
5. Device classification
6. Database update
7. Notification/Alert

### Device Information Structure
```go
type Device struct {
    ID          string            `json:"id"`
    IP          string            `json:"ip"`
    MAC         string            `json:"mac"`
    Vendor      string            `json:"vendor"`
    Type        string            `json:"type"`
    OS          string            `json:"os"`
    Services    []Service         `json:"services"`
    OpenPorts   []int             `json:"open_ports"`
    FirstSeen   time.Time         `json:"first_seen"`
    LastSeen    time.Time         `json:"last_seen"`
    Status      string            `json:"status"`
    Zone        string            `json:"zone"`
    Metadata    map[string]string `json:"metadata"`
}

type Service struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"`
    Name     string `json:"name"`
    Version  string `json:"version"`
    Banner   string `json:"banner"`
}
```

## Testing

### Device Discovery Testing
```bash
# Run discovery scan
flywall device-discovery scan --network 192.168.1.0/24

# Check discovered devices
flywall device-discovery list

# Get device details
flywall device-discovery get --ip 192.168.1.100

# Test fingerprinting
flywall device-discovery fingerprint --ip 192.168.1.100
```

### Integration Tests
- `device_discovery_test.sh`: Basic discovery
- `fingerprint_test.sh`: Device fingerprinting
- `classification_test.sh`: Device classification

## API Integration

### Device Discovery API
```bash
# Get all devices
curl -s "http://localhost:8080/api/devices"

# Get device by IP
curl -s "http://localhost:8080/api/devices/192.168.1.100"

# Search devices
curl -s "http://localhost:8080/api/devices?vendor=Cisco&type=network_device"

# Start scan
curl -X POST "http://localhost:8080/api/devices/scan" \
  -H "Content-Type: application/json" \
  -d '{
    "network": "192.168.1.0/24",
    "ports": [22, 80, 443]
  }'

# Get scan status
curl -s "http://localhost:8080/api/devices/scan/123/status"
```

### Device Management API
```bash
# Update device
curl -X PUT "http://localhost:8080/api/devices/192.168.1.100" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "server",
    "zone": "DMZ",
    "metadata": {
      "owner": "IT Team",
      "purpose": "Web Server"
    }
  }'

# Delete device
curl -X DELETE "http://localhost:8080/api/devices/192.168.1.100"

# Get device history
curl -s "http://localhost:8080/api/devices/192.168.1.100/history"
```

## Best Practices

1. **Network Impact**
   - Scan during off-peak hours
   - Limit concurrent scans
   - Use passive discovery when possible
   - Monitor network load

2. **Security**
   - Secure device information
   - Limit discovery scope
   - Authenticate API access
   - Audit discovery logs

3. **Accuracy**
   - Use multiple fingerprinting methods
   - Regularly update signatures
   - Validate classifications
   - Handle false positives

4. **Maintenance**
   - Clean stale devices
   - Update OUI database
   - Review classifications
   - Optimize scan schedules

## Troubleshooting

### Common Issues
1. **Devices not discovered**: Check network connectivity
2. **Wrong classification**: Update fingerprints
3. **High resource usage**: Adjust scan parameters
4. **Database errors**: Check permissions and space

### Debug Commands
```bash
# Check discovery status
flywall device-discovery status

# Test specific device
flywall device-discovery test --ip 192.168.1.100

# Check fingerprints
flywall device-discovery fingerprint-check --ip 192.168.1.100

# Monitor discovery
flywall device-discovery monitor
```

### Advanced Debugging
```bash
# Debug scan
flywall device-discovery debug --scan-id 123

# Check database
sqlite3 /var/lib/flywall/device_discovery.db "SELECT * FROM devices;"

# Validate fingerprints
flywall device-discovery validate-fingerprints

# Export devices
flywall device-discovery export --format json > devices.json
```

## Performance Considerations

- Scanning can generate significant traffic
- Database size grows with device count
- Fingerprinting adds CPU overhead
- Passive discovery is less intrusive

## Security Considerations

- Discovery reveals network topology
- Sensitive device information exposure
- Potential for network mapping attacks
- Need access controls

## Related Features

- [Network Monitoring](network-monitoring.md)
- [Asset Management](asset-management.md)
- [Security Policies](security-policies.md)
- [Compliance](compliance.md)
