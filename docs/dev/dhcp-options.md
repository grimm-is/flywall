# DHCP Options Implementation Guide

## Overview

Flywall provides comprehensive DHCP options support for:
- Standard DHCP options
- Custom vendor options
- Option filtering and manipulation
- Per-scope option configuration
- Dynamic option injection

## Architecture

### DHCP Options Components
1. **Option Parser**: Parses DHCP option fields
2. **Option Manager**: Manages option definitions
3. **Vendor Manager**: Handles vendor-specific options
4. **Filter Engine**: Filters and modifies options
5. **Injector**: Injects custom options

### Supported Options
- **Standard Options**: RFC-defined options (1-254)
- **Vendor Options**: Vendor-specific (option 43, 125)
- **Custom Options**: User-defined options
- **Encapsulated Options**: Nested option structures

## Configuration

### Basic DHCP Options Setup
```hcl
# DHCP options configuration
dhcp {
  enabled = true

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"
    router = "192.168.1.1"

    # Standard options
    options = {
      dns_servers = ["192.168.1.1", "8.8.8.8"]
      domain_name = "lan.example.com"
      lease_time = "24h"
      ntp_servers = ["192.168.1.1"]
    }
  }
}
```

### Advanced DHCP Options Configuration
```hcl
dhcp {
  enabled = true

  # Global option definitions
  option_definitions = [
    {
      name = "tftp_server"
      code = 66
      type = "string"
      description = "TFTP Server Address"
    },
    {
      name = "boot_file"
      code = 67
      type = "string"
      description = "Boot File Name"
    },
    {
      name = "voip_server"
      code = 150
      type = "ip_array"
      description = "VoIP Server Addresses"
    },
    {
      name = "custom_option"
      code = 222
      type = "hex"
      description = "Custom Configuration"
    }
  ]

  # Vendor classes
  vendor_classes = [
    {
      name = "voip_phone"
      identifier = "AastraIPPhone"

      # Vendor-specific options
      options = {
        tftp_server = "192.168.1.10"
        boot_file = "aastra.cfg"
        voip_server = ["192.168.1.20", "192.168.1.21"]
      }
    },
    {
      name = "cisco_phone"
      identifier = "Cisco IP Phone"

      options = {
        tftp_server = "192.168.1.10"
        boot_file = "SEP<mac>.cnf.xml"
        voip_server = ["192.168.1.20"]
      }
    },
    {
      name = "printer"
      identifier = "Printer"

      options = {
        tftp_server = "192.168.1.30"
        boot_file = "printer.cfg"
      }
    }
  ]

  # User classes
  user_classes = [
    {
      name = "pxe_client"
      identifier = "PXEClient"

      options = {
        tftp_server = "192.168.1.40"
        boot_file = "pxelinux.0"
        vendor_class_identifier = "PXEClient"
      }
    }
  ]

  scope "lan" {
    interface = "eth1"
    range_start = "192.168.1.100"
    range_end = "192.168.1.200"

    # Standard options
    options = {
      subnet_mask = "255.255.255.0"
      router = "192.168.1.1"
      dns_servers = ["192.168.1.1", "8.8.8.8"]
      domain_name = "lan.example.com"
      broadcast_address = "192.168.1.255"
      ntp_servers = ["192.168.1.1", "pool.ntp.org"]
      lease_time = "24h"
      renewal_time = "12h"
      rebinding_time = "21h"

      # Win/Mac integration
      netbios_name_servers = ["192.168.1.1"]
      netbios_node_type = "8"

      # SIP/VoIP
      sip_servers = ["192.168.1.20"]

      # Wireless
      wireless_access_points = ["192.168.1.50"]

      # Proxy
      wpad_url = "http://wpad.lan.example.com/wpad.dat"

      # Time
      time_servers = ["192.168.1.1"]
      time_offset = "-18000"  # EST offset

      # Authentication
      auth_servers = ["192.168.1.1"]

      # Logging
      log_servers = ["192.168.1.1"]

      # Static routes
      static_routes = [
        {
          destination = "10.0.0.0"
          netmask = "255.0.0.0"
          router = "192.168.1.254"
        },
        {
          destination = "192.168.100.0"
          netmask = "255.255.255.0"
          router = "192.168.1.253"
        }
      ]

      # Classless static routes
      classless_static_routes = [
        {
          destination = "10.0.0.0/8"
          router = "192.168.1.254"
        },
        {
          destination = "172.16.0.0/12"
          router = "192.168.1.254"
        }
      ]
    }

    # Per-host options
    host_options = [
      {
        mac = "00:11:22:33:44:55"
        hostname = "server01"
        ip_address = "192.168.1.10"

        options = {
          tftp_server = "192.168.1.10"
          boot_file = "server01.cfg"
          dns_servers = ["192.168.1.10", "8.8.8.8"]
        }
      },
      {
        mac = "aa:bb:cc:dd:ee:ff"
        hostname = "voip01"
        ip_address = "192.168.1.100"

        options = {
          tftp_server = "192.168.1.20"
          boot_file = "voip.cfg"
          voip_server = ["192.168.1.20"]
        }
      }
    ]
  }

  scope "guest" {
    interface = "eth2"
    range_start = "192.168.200.100"
    range_end = "192.168.200.200"

    # Restricted options for guests
    options = {
      subnet_mask = "255.255.255.0"
      router = "192.168.200.1"
      dns_servers = ["8.8.8.8", "1.1.1.1"]  # External DNS only
      domain_name = "guest.example.com"
      lease_time = "2h"

      # Captive portal
      wpad_url = "http://192.168.200.1/captive-portal"

      # Block internal services
      netbios_name_servers = []
      sip_servers = []
      log_servers = []
    }

    # Option filtering
    filter = {
      # Block these options
      block = [43, 125, 150, 208, 209]

      # Allow only these
      allow = [1, 3, 6, 15, 51, 54, 119]
    }
  }
}
```

### Option Filtering and Manipulation
```hcl
dhcp {
  enabled = true

  # Global option filters
  filters = [
    {
      name = "security_filter"
      description = "Remove potentially dangerous options"

      # Match criteria
      match = {
        source = "any"
        vendor_class = "any"
      }

      # Actions
      actions = [
        {
          type = "remove"
          options = [43, 125, 150]  # Vendor-specific options
        },
        {
          type = "replace"
          option = "domain_search"
          value = ["guest.example.com"]
        }
      ]
    },
    {
      name = "voip_filter"
      description = "Add VoIP options for phones"

      match = {
        vendor_class = ["AastraIPPhone", "Cisco IP Phone"]
      }

      actions = [
        {
          type = "add"
          options = {
            tftp_server = "192.168.1.10"
            voip_server = ["192.168.1.20"]
          }
        }
      ]
    }
  ]

  # Option transformations
  transformations = [
    {
      name = "dns_override"
      description = "Override DNS based on client"

      match = {
        mac_prefix = ["00:11:22", "aa:bb:cc"]
      }

      transform = {
        option = "dns_servers"
        value = ["192.168.1.10", "192.168.1.11"]
      }
    },
    {
      name = "lease_adjustment"
      description = "Adjust lease time based on device type"

      match = {
        vendor_class = "Printer"
      }

      transform = {
        option = "lease_time"
        value = "7d"  # Longer lease for printers
      }
    }
  ]
}
```

### Dynamic Options
```hcl
dhcp {
  enabled = true

  # Dynamic option injection
  dynamic_options = {
    enabled = true

    # HTTP-based options
    http = {
      enabled = true
      url = "http://config.example.com/dhcp-options"

      # Authentication
      auth = {
        username = "dhcp"
        password = "secret"
      }

      # Update interval
      update_interval = "5m"

      # Cache
      cache = {
        ttl = "10m"
        max_size = 1000
      }
    }

    # Database-based options
    database = {
      enabled = true
      type = "mysql"
      connection = "dhcp:password@tcp(localhost:3306)/dhcp"

      # Query template
      query = "SELECT option_code, option_value FROM dhcp_options WHERE mac_address = ? OR hostname = ?"
    }

    # Script-based options
    script = {
      enabled = true
      path = "/etc/flywall/dhcp-options.sh"

      # Script arguments
      args = ["--mac", "%MAC%", "--hostname", "%HOSTNAME%"]

      # Timeout
      timeout = "5s"
    }
  }

  # Conditional options
  conditional_options = [
    {
      name = "time_based_options"
      description = "Different options based on time"

      conditions = [
        {
          time_range = "09:00-17:00"
          days = ["mon", "tue", "wed", "thu", "fri"]

          options = {
            dns_servers = ["192.168.1.1", "8.8.8.8"]
            proxy_auto_config = "http://wpad.example.com/workday.wpad"
          }
        },
        {
          time_range = "17:00-09:00"

          options = {
            dns_servers = ["8.8.8.8", "1.1.1.1"]
            proxy_auto_config = "http://wpad.example.com/afterhours.wpad"
          }
        }
      ]
    },
    {
      name = "load_balancing"
      description = "Load balance DNS servers"

      conditions = [
        {
          expression = "hash(%MAC%) % 2 == 0"

          options = {
            dns_servers = ["192.168.1.1", "192.168.1.2"]
          }
        },
        {
          expression = "hash(%MAC%) % 2 == 1"

          options = {
            dns_servers = ["192.168.1.3", "192.168.1.4"]
          }
        }
      ]
    }
  ]
}
```

## Implementation Details

### DHCP Option Format
```
Option Code: 1 byte
Option Length: 1 byte
Option Data: Variable length
```

### Common Option Codes
- 1: Subnet Mask
- 3: Router
- 6: DNS Servers
- 15: Domain Name
- 43: Vendor Specific
- 51: Lease Time
- 53: Message Type
- 54: Server Identifier
- 55: Parameter Request List
- 60: Vendor Class Identifier
- 61: Client Identifier
- 66: TFTP Server
- 67: Boot File Name

## Testing

### DHCP Options Testing
```bash
# Test DHCP client
dhclient -d eth1

# Check received options
dhclient -r eth1 && dhclient -d eth1 | grep "DHCPACK from"

# Test with specific vendor class
dhclient -d -v eth1 -sf /dev/null -cf /dev/null | grep "Vendor"

# Monitor DHCP traffic
tcpdump -i eth1 -vv port 67 or port 68
```

### Integration Tests
- `dhcp_options_test.sh`: Basic options
- `vendor_class_test.sh`: Vendor-specific options
- `filter_test.sh`: Option filtering

## API Integration

### DHCP Options API
```bash
# Get DHCP options
curl -s "http://localhost:8080/api/dhcp/options"

# Get scope options
curl -s "http://localhost:8080/api/dhcp/scopes/lan/options"

# Update options
curl -X PUT "http://localhost:8080/api/dhcp/scopes/lan/options" \
  -H "Content-Type: application/json" \
  -d '{
    "dns_servers": ["192.168.1.1", "1.1.1.1"],
    "lease_time": "12h"
  }'

# Get host options
curl -s "http://localhost:8080/api/dhcp/hosts/00:11:22:33:44:55/options"
```

### Option Statistics API
```bash
# Get option statistics
curl -s "http://localhost:8080/api/dhcp/options/stats"

# Get most used options
curl -s "http://localhost:8080/api/dhcp/options/popular"

# Get vendor class statistics
curl -s "http://localhost:8080/api/dhcp/vendor-classes/stats"
```

## Best Practices

1. **Option Management**
   - Use standard options when possible
   - Document custom options
   - Test vendor-specific options
   - Keep options consistent

2. **Security**
   - Filter dangerous options
   - Validate option values
   - Monitor for abuse
   - Use secure vendor options

3. **Performance**
   - Limit option size
   - Cache dynamic options
   - Optimize filters
   - Monitor processing time

4. **Compatibility**
   - Test with various clients
   - Handle unknown options gracefully
   - Provide fallback options
   - Follow RFC specifications

## Troubleshooting

### Common Issues
1. **Options not received**: Check option codes and formatting
2. **Vendor options ignored**: Verify vendor class matching
3. **Client rejects options**: Validate option values
4. **Performance issues**: Optimize filters and processing

### Debug Commands
```bash
# Check DHCP options
flywall dhcp options show

# Test specific client
flywall dhcp test --mac 00:11:22:33:44:55

# Monitor option processing
flywall dhcp monitor --options

# Validate configuration
flywall dhcp validate
```

### Advanced Debugging
```bash
# Debug option processing
flywall dhcp debug --options --verbose

# Check vendor class matching
flywall dhcp debug --vendor-class "AastraIPPhone"

# Test filters
flywall dhcp test-filter --filter security_filter

# Export options
flywall dhcp export-options --format json > options.json
```

## Performance Considerations

- Option processing adds minimal overhead
- Complex filters increase CPU usage
- Dynamic options need caching
- Large option sizes affect packets

## Security Considerations

- Options can carry malicious data
- Vendor options may be untrusted
- Option injection attacks possible
- Need input validation

## Related Features

- [DHCP Server](dhcp-server.md)
- [DHCP Lease Management](dhcp-lease-mgmt.md)
- [Device Discovery](device-discovery.md)
- [Network Policies](zones-policies.md)
