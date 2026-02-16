# CLI Tools Implementation Guide

## Overview

Flywall provides a comprehensive command-line interface for:
- Configuration management
- System monitoring
- Debugging and troubleshooting
- Batch operations
- Scripting support

## CLI Commands

### Configuration Commands

#### `flywall config`
```bash
# Show current configuration
flywall config show

# Get specific section
flywall config get interfaces
flywall config get zones.wan

# Validate configuration
flywall config validate /path/to/config.hcl

# Apply configuration
flywall config apply /path/to/config.hcl

# Dry-run apply
flywall config apply --dry-run /path/to/config.hcl

# Show configuration diff
flywall config diff /path/to/new.hcl

# Edit configuration
flywall config edit

# Backup configuration
flywall config backup

# Restore configuration
flywall config restore backup-20231201.hcl

# Check configuration
flywall check /path/to/config.hcl
```

#### `flywall show`
```bash
# Show system status
flywall show status

# Show interfaces
flywall show interfaces

# Show zones
flywall show zones

# Show policies
flywall show policies

# Show NAT rules
flywall show nat

# Show routing table
flywall show routes

# Show DHCP status
flywall show dhcp

# Show DNS status
flywall show dns

# Show VPN status
flywall show vpn

# Show learning status
flywall show learning

# Show HA status
flywall show ha
```

### Monitoring Commands

#### `flywall monitor`
```bash
# Real-time monitoring
flywall monitor

# Monitor specific interface
flywall monitor --interface eth0

# Monitor with filters
flywall monitor --filter "port=22"

# Monitor packet flow
flywall monitor --packet-flow

# Monitor connection tracking
flywall monitor --conntrack

# Export to file
flywall monitor --output /tmp/monitor.log
```

#### `flywall stats`
```bash
# Show statistics summary
flywall stats

# Interface statistics
flywall stats interfaces

# Policy statistics
flywall stats policies

# Connection statistics
flywall stats connections

# Performance metrics
flywall stats performance

# Historical stats
flywall stats --since "1h ago"

# Stats in JSON format
flywall stats --format json
```

### Debugging Commands

#### `flywall debug`
```bash
# Enable debug mode
flywall debug on

# Disable debug mode
flywall debug off

# Debug specific component
flywall debug component nat

# Debug with verbosity
flywall debug --level trace

# Debug packet trace
flywall debug trace --src 192.168.1.100 --dst 8.8.8.8

# Debug rule evaluation
flywall debug rules --policy wan_to_lan

# Generate debug report
flywall debug report --output /tmp/debug.txt
```

#### `flywall test`
```bash
# Test configuration
flywall test config

# Test connectivity
flywall test connectivity --target 8.8.8.8

# Test DNS resolution
flywall test dns --name example.com

# Test DHCP server
flywall test dhcp

# Test VPN connectivity
flywall test vpn --peer 10.200.0.2

# Run all tests
flywall test all
```

### Management Commands

#### `flywall service`
```bash
# Start service
flywall service start

# Stop service
flywall service stop

# Restart service
flywall service restart

# Reload configuration
flywall service reload

# Show service status
flywall service status

# Enable autostart
flywall service enable

# Disable autostart
flywall service disable
```

#### `flywall backup`
```bash
# Create backup
flywall backup create

# List backups
flywall backup list

# Restore backup
flywall backup restore backup-20231201-120000

# Delete backup
flywall backup delete backup-20231201-120000

# Schedule backup
flywall backup schedule --daily 02:00

# Export backup
flywall backup export --output /tmp/flywall-backup.tar
```

### Advanced Commands

#### `flywall migrate`
```bash
# Check migration status
flywall migrate status

# Run migration
flywall migrate up

# Rollback migration
flywall migrate down

# Create new migration
flywall migrate create add_new_feature

# Validate migration
flywall migrate validate
```

#### `flywall plugin`
```bash
# List plugins
flywall plugin list

# Install plugin
flywall plugin install flywall-threat-intel

# Uninstall plugin
flywall plugin uninstall flywall-threat-intel

# Enable plugin
flywall plugin enable threat-intel

# Disable plugin
flywall plugin disable threat-intel

# Update plugin
flywall plugin update threat-intel
```

## Command Options

### Global Options
```bash
# Configuration file
flywall --config /etc/flywall/custom.hcl show status

# Output format
flywall --format json show interfaces

# Verbose output
flywall --verbose config validate

# Quiet mode
flywall --quiet service restart

# Help
flywall --help
flywall config --help

# Version
flywall --version
```

### Filtering Options
```bash
# Filter by interface
flywall show interfaces --filter "name=eth*"

# Filter by zone
flywall show policies --filter "zone=wan"

# Filter by time
flywall stats --since "2023-12-01"

# Filter by label
flywall show connections --filter "labels.protocol=tcp"
```

## Scripting Support

### JSON Output
```bash
# Get interfaces as JSON
flywall --format json show interfaces > interfaces.json

# Parse with jq
flywall --format json show stats | jq '.interfaces.eth0.bytes_in'

# Use in scripts
#!/bin/bash
STATUS=$(flywall --format json show status | jq '.health')
if [ "$STATUS" = "\"healthy\"" ]; then
    echo "System is healthy"
fi
```

### Batch Operations
```bash
# Apply multiple configs
for config in configs/*.hcl; do
    flywall config validate "$config"
done

# Bulk interface changes
for iface in eth1 eth2 eth3; do
    flywall interface set "$iface" mtu 9000
done

# Health check script
#!/bin/bash
check_health() {
    local component=$1
    local status=$(flywall --format json show "$component" | jq '.status')
    echo "$component: $status"
}

for component in interfaces zones policies; do
    check_health "$component"
done
```

### Completion
```bash
# Enable bash completion
source <(flywall completion bash)

# Enable zsh completion
source <(flywall completion zsh)

# Install completion system-wide
flywall completion install --shell bash
```

## Examples

### Common Workflows
```bash
# 1. Update configuration safely
flywall config backup
flywall config validate new-config.hcl
flywall config apply --dry-run new-config.hcl
flywall config apply new-config.hcl

# 2. Troubleshoot connectivity
flywall show interfaces
flywall show routes
flywall monitor --interface eth0
flywall debug trace --src 192.168.1.100 --dst 8.8.8.8

# 3. Monitor system health
flywall show status
flywall stats
flywall monitor --filter "severity=error"

# 4. Manage VPN connections
flywall show vpn
flywall vpn peer add client1 --public-key "..." --allowed-ips "10.200.0.10/32"
flywall vpn export client1 > client1.conf
```

### Automation Examples
```bash
# Daily health check
#!/bin/bash
DATE=$(date +%Y%m%d)
REPORT="/tmp/health-report-$DATE.txt"

{
    echo "=== Flywall Health Report ==="
    echo "Date: $(date)"
    echo ""

    flywall show status
    echo ""

    flywall stats --since "24h"
    echo ""

    flywall show interfaces | grep -E "(UP|DOWN)"
} > "$REPORT"

mail -s "Flywall Health Report" admin@example.com < "$REPORT"

# Configuration deployment
#!/bin/bash
CONFIG_DIR="/etc/flywall/configs"
BACKUP_DIR="/var/backups/flywall"

# Create backup
flywall backup create

# Deploy new configs
for config in "$CONFIG_DIR"/*.hcl; do
    echo "Deploying $config..."
    flywall config validate "$config" || exit 1
    flywall config apply "$config"
done

# Verify deployment
flywall test all
```

## Best Practices

1. **Configuration Management**
   - Always validate before applying
   - Use dry-run to preview changes
   - Keep regular backups
   - Use version control

2. **Monitoring**
   - Use filters for focused monitoring
   - Export data for analysis
   - Set up automated alerts
   - Log important events

3. **Scripting**
   - Use JSON output for parsing
   - Handle errors gracefully
   - Add logging for debugging
   - Test scripts thoroughly

4. **Security**
   - Use sudo only when necessary
   - Secure configuration files
   - Audit command usage
   - Use authentication for remote access

## Troubleshooting

### Common Issues
1. **Command not found**: Check PATH and installation
2. **Permission denied**: Use sudo or check permissions
3. **Config parse error**: Validate syntax and schema
4. **Service not responding**: Check service status

### Debug Commands
```bash
# Check CLI version
flywall --version

# Validate installation
flywall doctor

# Check permissions
flywall check-permissions

# Debug command execution
flywall --verbose --debug command
```

## Related Features

- [Configuration Management](config-management.md)
- [API Reference](api-reference.md)
- [Monitoring](monitoring.md)
- [Troubleshooting Guide](troubleshooting.md)
