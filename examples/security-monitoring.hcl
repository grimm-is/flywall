# Security and Monitoring Configuration
#
# This example demonstrates:
# 1. Rule learning (TOFU mode)
# 2. Anomaly detection
# 3. Notifications (alerts)
# 4. GeoIP blocking
# 5. Threat intelligence
# 6. Audit logging

schema_version = "1.0"
ip_forwarding = true

# -----------------------------------------------------------------------------
# Interfaces
# -----------------------------------------------------------------------------

interface "eth0" {
  description = "WAN"
  zone        = "wan"
  dhcp        = true
}

interface "eth1" {
  description = "LAN"
  zone        = "lan"
  ipv4        = ["192.168.1.1/24"]
}

# -----------------------------------------------------------------------------
# GeoIP Configuration
# -----------------------------------------------------------------------------

geoip {
  enabled       = true
  database_path = "/var/lib/flywall/geoip/GeoLite2-Country.mmdb"
  auto_update   = true
  license_key   = "${MAXMIND_LICENSE_KEY}"
}

# -----------------------------------------------------------------------------
# Protection with GeoIP Blocking
# -----------------------------------------------------------------------------

protection "wan_protection" {
  interface            = "eth0"
  enabled              = true
  anti_spoofing        = true
  bogon_filtering      = true
  private_filtering    = true
  invalid_packets      = true
  syn_flood_protection = true
  syn_flood_rate       = 25
  syn_flood_burst      = 50
  icmp_rate_limit      = true
  icmp_rate            = 10
  icmp_burst           = 20
  new_conn_rate_limit  = true
  new_conn_rate        = 100
  new_conn_burst       = 200
  port_scan_protection = true
  port_scan_threshold  = 10

  # GeoIP blocking
  geo_blocking     = true
  blocked_countries = ["CN", "RU", "KP", "IR"]
}

# -----------------------------------------------------------------------------
# Threat Intelligence Feeds
# -----------------------------------------------------------------------------

threat_intel {
  enabled  = true
  interval = "1h"

  source "abuse-ch" {
    url    = "https://feodotracker.abuse.ch/downloads/ipblocklist.txt"
    format = "text"
  }

  source "emerging-threats" {
    url    = "https://rules.emergingthreats.net/fwrules/emerging-Block-IPs.txt"
    format = "text"
  }
}

# -----------------------------------------------------------------------------
# Rule Learning (TOFU Mode)
# -----------------------------------------------------------------------------

rule_learning {
  enabled        = true
  learning_mode  = true
  log_group      = 100
  rate_limit     = "10/minute"
  cache_size     = 10000
  retention_days = 30

  # Inline mode for zero packet loss during learning
  inline_mode = false

  # Networks to exclude from learning
  ignore_networks = [
    "224.0.0.0/4",
    "255.255.255.255/32"
  ]
}

# -----------------------------------------------------------------------------
# Anomaly Detection
# -----------------------------------------------------------------------------

anomaly_detection {
  enabled            = true
  baseline_window    = "7d"
  min_samples        = 100
  spike_stddev       = 3.0
  drop_stddev        = 2.0
  alert_cooldown     = "15m"
  port_scan_threshold = 15
}

# -----------------------------------------------------------------------------
# Notifications
# -----------------------------------------------------------------------------

notifications {
  enabled = true

  # Email notifications
  channel "email-admin" {
    type      = "email"
    level     = "critical"
    enabled   = true
    smtp_host = "smtp.example.com"
    smtp_port = 587
    smtp_user = "alerts@example.com"
    smtp_password = "${SMTP_PASSWORD}"
    from      = "flywall@example.com"
    to        = ["admin@example.com"]
  }

  # Slack notifications
  channel "slack-security" {
    type        = "slack"
    level       = "warning"
    enabled     = true
    webhook_url = "${SLACK_WEBHOOK_URL}"
    channel     = "#security-alerts"
    username    = "Flywall"
  }

  # Pushover for mobile
  channel "pushover-admin" {
    type      = "pushover"
    level     = "critical"
    enabled   = true
    api_token = "${PUSHOVER_API_TOKEN}"
    user_key  = "${PUSHOVER_USER_KEY}"
    priority  = 1
    sound     = "siren"
  }

  # ntfy.sh for self-hosted
  channel "ntfy" {
    type    = "ntfy"
    level   = "info"
    enabled = true
    server  = "https://ntfy.example.com"
    topic   = "flywall-alerts"
  }

  # Discord webhook
  channel "discord-logs" {
    type        = "discord"
    level       = "info"
    enabled     = true
    webhook_url = "${DISCORD_WEBHOOK_URL}"
  }

  # Alert rules
  rule "high-traffic-spike" {
    enabled   = true
    condition = "traffic_spike > 300%"
    severity  = "warning"
    channels  = ["slack-security", "email-admin"]
    cooldown  = "1h"
  }

  rule "port-scan-detected" {
    enabled   = true
    condition = "port_scan_detected"
    severity  = "critical"
    channels  = ["pushover-admin", "slack-security"]
    cooldown  = "15m"
  }

  rule "new-device-learned" {
    enabled   = true
    condition = "new_rule_learned"
    severity  = "info"
    channels  = ["ntfy", "discord-logs"]
    cooldown  = "5m"
  }

  rule "uplink-failover" {
    enabled   = true
    condition = "uplink_changed"
    severity  = "warning"
    channels  = ["slack-security", "pushover-admin"]
    cooldown  = "5m"
  }
}

# -----------------------------------------------------------------------------
# Audit Logging
# -----------------------------------------------------------------------------

audit {
  enabled        = true
  log_file       = "/var/log/flywall/audit.log"
  log_config_changes = true
  log_rule_changes   = true
  log_admin_access   = true
  retention_days = 90
}

# -----------------------------------------------------------------------------
# Firewall Policies with GeoIP and Logging
# -----------------------------------------------------------------------------

policy "wan" "self" {
  rule "block-geoip-countries" {
    source_country = "CN"
    action         = "drop"
    log            = true
    log_prefix     = "GEOIP-BLOCK: "
  }

  rule "allow-ssh-us-only" {
    proto          = "tcp"
    dest_port      = 22
    source_country = "US"
    action         = "accept"
    log            = true
    log_prefix     = "SSH-ACCESS: "
  }

  rule "allow-icmp" {
    proto  = "icmp"
    action = "accept"
    limit  = "5/second"
  }
}

policy "lan" "wan" {
  action = "accept"
}

policy "lan" "self" {
  rule "allow-dns" {
    proto     = "udp"
    dest_port = 53
    action    = "accept"
  }

  rule "allow-dhcp" {
    proto     = "udp"
    dest_port = 67
    action    = "accept"
  }

  rule "allow-ssh" {
    proto     = "tcp"
    dest_port = 22
    action    = "accept"
    log       = true
    log_prefix = "LAN-SSH: "
  }

  rule "allow-web-ui" {
    proto     = "tcp"
    dest_port = 443
    action    = "accept"
  }
}

# -----------------------------------------------------------------------------
# NAT
# -----------------------------------------------------------------------------

nat "masquerade" {
  type          = "masquerade"
  out_interface = "eth0"
}
