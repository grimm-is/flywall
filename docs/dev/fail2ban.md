# Fail2Ban Integration Implementation Guide

## Overview

Flywall provides Fail2Ban integration for:
- Dynamic IP blocking based on failed attempts
- Brute force protection
- Pattern-based detection
- Custom ban policies
- Integration with firewall rules

## Architecture

### Fail2Ban Components
1. **Log Monitor**: Monitors logs for patterns
2. **Pattern Matcher**: Matches failure patterns
3. **Ban Manager**: Manages IP bans
4. **Jail Manager**: Configures jails
5. **Action Executor**: Executes ban actions

### Supported Services
- SSH
- FTP
- HTTP/HTTPS
- SMTP
- DNS
- Custom services

## Configuration

### Basic Fail2Ban Setup
```hcl
# Fail2Ban configuration
fail2ban {
  enabled = true

  # Global settings
  global = {
    bantime = "10m"
    findtime = "10m"
    maxretry = 5
  }

  # Jails
  jails = [
    {
      name = "ssh"
      enabled = true
      filter = "sshd"
      action = "flywall"
      logpath = "/var/log/auth.log"
      maxretry = 3
      bantime = "1h"
    }
  ]
}
```

### Advanced Fail2Ban Configuration
```hcl
fail2ban {
  enabled = true

  # Global settings
  global = {
    # Ban settings
    bantime = "10m"
    bantime_increment = true
    bantime_factor = 2
    maxbantime = "7d"

    # Detection settings
    findtime = "10m"
    maxretry = 5

    # Backend
    backend = "auto"

    # Database
    database = "/var/lib/flywall/fail2ban.db"

    # Ignore IPs
    ignoreip = [
      "127.0.0.1/8",
      "192.168.0.0/16",
      "10.0.0.0/8"
    ]
  }

  # Actions
  actions = [
    {
      name = "flywall"
      type = "firewall"

      # Ban action
      ban = {
        type = "block"
        table = "inet"
        chain = "input"
        target = "drop"
      }

      # Unban action
      unban = {
        type = "unblock"
      }

      # Check action
      check = {
        type = "check"
      }

      # Properties
      properties = {
        bantime = "bantime"
        ip = "ip"
      }
    },
    {
      name = "mail"
      type = "notification"

      # Email settings
      mail = {
        enabled = true
        from = "fail2ban@example.com"
        to = "admin@example.com"
        subject = "[Fail2Ban] <name>: banned <ip>"

        # SMTP settings
        smtp = {
          host = "smtp.example.com"
          port = 587
          tls = true
          username = "fail2ban"
          password = "password"
        }
      }
    }
  ]

  # Filters
  filters = [
    {
      name = "sshd"
      type = "regex"

      # Failure patterns
      failregex = [
        "Failed password for .* from <HOST> port \\d+ ssh2",
        "Invalid user .* from <HOST>",
        "User .* from <HOST> not allowed because not listed in AllowUsers",
        "POSSIBLE BREAK-IN ATTEMPT!.* from <HOST>"
      ]

      # Ignore patterns
      ignoreregex = [
        ".* from 192.168.*",
        ".* from 10.*"
      ]
    },
    {
      name = "apache-auth"
      type = "regex"

      failregex = [
        "[] client <HOST> .* authentication failure for \".*\"",
        "[] client <HOST> .* user .* authentication failure"
      ]
    },
    {
      name = "dns"
      type = "regex"

      failregex = [
        ".* query: .* blocked for <HOST>",
        ".* client <HOST>.* query denied"
      ]
    }
  ]

  # Jails
  jails = [
    {
      name = "sshd"
      enabled = true

      # Filter and action
      filter = "sshd"
      action = ["flywall", "mail"]

      # Log path
      logpath = [
        "/var/log/auth.log",
        "/var/log/secure"
      ]

      # Settings
      maxretry = 3
      bantime = "1h"
      findtime = "30m"

      # Ignore IPs
      ignoreip = [
        "192.168.1.100",  # Admin workstation
        "10.0.0.50"       # Monitoring server
      ]
    },
    {
      name = "apache-auth"
      enabled = true

      filter = "apache-auth"
      action = "flywall"
      logpath = "/var/log/apache2/error.log"
      maxretry = 5
      bantime = "30m"
    },
    {
      name = "dns"
      enabled = true

      filter = "dns"
      action = "flywall"
      logpath = "/var/log/flywall/dns.log"
      maxretry = 10
      bantime = "15m"
      findtime = "5m"
    },
    {
      name = "recidive"
      enabled = true

      # Repeat offender jail
      filter = "recidive"
      action = "flywall"
      logpath = "/var/log/fail2ban.log"
      bantime = "1w"
      findtime = "1d"
      maxretry = 5
    }
  ]
}
```

### Custom Jails
```hcl
fail2ban {
  enabled = true

  # Custom jail for web login
  jail "web-login" {
    enabled = true

    # Custom filter
    filter = {
      type = "regex"
      failregex = [
        ".* POST /login .* 401 .* client <HOST>",
        ".* POST /api/auth .* 403 .* client <HOST>",
        ".* failed login attempt from <HOST>"
      ]
    }

    action = "flywall"
    logpath = "/var/log/flywall/api.log"
    maxretry = 5
    bantime = "2h"
    findtime = "15m"
  }

  # Jail for VPN authentication
  jail "vpn-auth" {
    enabled = true

    filter = {
      type = "regex"
      failregex = [
        ".* wireguard.* authentication failed for peer .* from <HOST>",
        ".* vpn.* invalid credentials from <HOST>"
      ]
    }

    action = ["flywall", "mail"]
    logpath = "/var/log/flywall/vpn.log"
    maxretry = 3
    bantime = "6h"
    findtime = "1h"
  }

  # Jail for API abuse
  jail "api-abuse" {
    enabled = true

    filter = {
      type = "regex"
      failregex = [
        ".* rate limit exceeded for <HOST>",
        ".* API key invalid from <HOST>",
        ".* too many requests from <HOST>"
      ]
    }

    action = "flywall"
    logpath = "/var/log/flywall/api.log"
    maxretry = 20
    bantime = "1h"
    findtime = "5m"

    # Per-IP rate limiting
    per_ip_rate_limit = {
      enabled = true
      requests_per_minute = 60
      burst = 100
    }
  }
}
```

### Advanced Ban Policies
```hcl
fail2ban {
  enabled = true

  # Ban policies
  ban_policies = [
    {
      name = "progressive"
      description = "Progressive ban duration"

      # Progressive ban times
      ban_times = [
        { offense: 1, duration: "10m" },
        { offense: 2, duration: "1h" },
        { offense: 3, duration: "6h" },
        { offense: 4, duration: "1d" },
        { offense: 5, duration: "1w" }
      ]

      # Reset after
      reset_after = "30d"
    },
    {
      name = "geographic"
      description = "Geographic-based bans"

      # Country-specific policies
      countries = [
        {
          code = "CN"
          bantime = "1h"
          maxretry = 2
        },
        {
          code = "RU"
          bantime = "30m"
          maxretry = 3
        },
        {
          code = "US"
          bantime = "10m"
          maxretry = 5
        }
      ]
    }
  ]

  # Apply policies to jails
  jail "sshd" {
    enabled = true
    filter = "sshd"
    action = "flywall"
    logpath = "/var/log/auth.log"

    # Apply progressive policy
    ban_policy = "progressive"
  }

  jail "web-login" {
    enabled = true
    filter = "web-login"
    action = "flywall"
    logpath = "/var/log/flywall/api.log"

    # Apply geographic policy
    ban_policy = "geographic"
  }
}
```

### Integration with Flywall Features
```hcl
fail2ban {
  enabled = true

  # Integration with IP sets
  ipset_integration = {
    enabled = true

    # IP set names
    ipsets = {
      banned = "fail2ban_banned"
      permanent = "fail2ban_permanent"
      temporary = "fail2ban_temp"
    }

    # Automatic cleanup
    cleanup = {
      enabled = true
      interval = "1h"
    }
  }

  # Integration with GeoIP
  geoip_integration = {
    enabled = true

    # Country-based policies
    country_policies = [
      {
        countries = ["CN", "RU", "KP"]
        action = "ban_immediately"
        bantime = "24h"
      },
      {
        countries = ["US", "CA", "GB"]
        action = "normal"
      }
    ]
  }

  # Integration with learning engine
  learning_integration = {
    enabled = true

    # Learn from patterns
    learn_patterns = true

    # Suggest new jails
    suggest_jails = true

    # Auto-create rules
    auto_rules = false
  }
}
```

## Implementation Details

### Ban Process
1. Monitor log files
2. Match failure patterns
3. Count failures per IP
4. Check threshold
5. Apply ban action
6. Schedule unban
7. Log ban event

### Pattern Matching
```regex
# Example SSH patterns
Failed password for .* from <HOST> port \d+ ssh2
Invalid user .* from <HOST>
POSSIBLE BREAK-IN ATTEMPT!.* from <HOST>

# Variables
<HOST> - IP address
<port> - Port number
<user> - Username
<failures> - Number of failures
```

## Testing

### Fail2Ban Testing
```bash
# Test SSH jail
ssh invalid@localhost

# Check ban status
flywall fail2ban status sshd

# Check banned IPs
flywall fail2ban banned

# Test unban
flywall fail2ban unban 192.168.1.100
```

### Integration Tests
- `fail2ban_test.sh`: Basic Fail2Ban functionality
- `jail_test.sh`: Jail configuration
- `pattern_test.sh`: Pattern matching

## API Integration

### Fail2Ban API
```bash
# Get Fail2Ban status
curl -s "http://localhost:8080/api/fail2ban/status"

# List jails
curl -s "http://localhost:8080/api/fail2ban/jails"

# Get jail details
curl -s "http://localhost:8080/api/fail2ban/jails/sshd"

# Ban IP manually
curl -X POST "http://localhost:8080/api/fail2ban/ban" \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "192.168.1.100",
    "jail": "sshd",
    "duration": "1h"
  }'

# Unban IP
curl -X DELETE "http://localhost:8080/api/fail2ban/ban/192.168.1.100"
```

### Statistics API
```bash
# Get statistics
curl -s "http://localhost:8080/api/fail2ban/stats"

# Get jail statistics
curl -s "http://localhost:8080/api/fail2ban/jails/sshd/stats"

# Get banned IPs
curl -s "http://localhost:8080/api/fail2ban/banned"
```

## Best Practices

1. **Jail Configuration**
   - Start with conservative settings
   - Test patterns before deployment
   - Monitor false positives
   - Adjust thresholds as needed

2. **Performance**
   - Optimize regex patterns
   - Limit log file sizes
   - Use efficient backends
   - Monitor resource usage

3. **Security**
   - Protect against log injection
   - Validate IP addresses
   - Secure ban actions
   - Review bans regularly

4. **Maintenance**
   - Regular jail reviews
   - Update patterns
   - Clean old bans
   - Monitor effectiveness

## Troubleshooting

### Common Issues
1. **No bans occurring**: Check log paths and patterns
2. **False positives**: Adjust regex patterns
3. **High resource usage**: Optimize patterns
4. **Bans not working**: Check action configuration

### Debug Commands
```bash
# Check Fail2Ban status
flywall fail2ban status

# Test regex
flywall fail2ban regex-test --filter sshd --log /var/log/auth.log

# Check jail status
flywall fail2ban jail-status sshd

# Monitor logs
tail -f /var/log/fail2ban.log
```

### Advanced Debugging
```bash
# Debug specific jail
flywall fail2ban debug --jail sshd

# Test pattern matching
fail2ban-regex /var/log/auth.log "Failed password for .* from <HOST>"

# Check database
sqlite3 /var/lib/flywall/fail2ban.db "SELECT * FROM bans;"

# Real-time monitoring
watch -n 1 'flywall fail2ban status'
```

## Performance Considerations

- Regex matching can be CPU intensive
- Log file size affects performance
- Number of jails impacts memory
- Database operations add overhead

## Security Considerations

- Log file permissions
- Pattern injection risks
- Ban action security
- Database protection

## Related Features

- [IP Sets & Blocklists](ipsets-blocklists.md)
- [Protection Features](protection-features.md)
- [GeoIP Integration](geoip.md)
- [Security Policies](security-policies.md)
