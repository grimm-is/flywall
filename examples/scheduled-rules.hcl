# Scheduled Rules Configuration
#
# This example demonstrates:
# 1. Time-based firewall rules
# 2. Scheduled rule blocks
# 3. Day-of-week restrictions
# 4. Parental controls pattern

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

interface "eth2" {
  description = "Kids Network"
  zone        = "kids"
  ipv4        = ["192.168.50.1/24"]
}

# -----------------------------------------------------------------------------
# Scheduler Configuration
# -----------------------------------------------------------------------------

scheduler {
  enabled  = true
  timezone = "America/New_York"
}

# -----------------------------------------------------------------------------
# Scheduled Rules
# -----------------------------------------------------------------------------

# Block kids internet access during school hours (weekdays 8am-3pm)
scheduled_rule "school-hours-block" {
  enabled    = true
  zone_from  = "kids"
  zone_to    = "wan"
  action     = "drop"
  time_start = "08:00"
  time_end   = "15:00"
  days       = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
  log        = true
  log_prefix = "SCHOOL-BLOCK: "
}

# Block kids internet after bedtime (10pm-6am)
scheduled_rule "bedtime-block" {
  enabled    = true
  zone_from  = "kids"
  zone_to    = "wan"
  action     = "drop"
  time_start = "22:00"
  time_end   = "06:00"
  days       = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday"]
  log        = true
  log_prefix = "BEDTIME-BLOCK: "
}

# Weekend bedtime is later (11pm-7am)
scheduled_rule "weekend-bedtime-block" {
  enabled    = true
  zone_from  = "kids"
  zone_to    = "wan"
  action     = "drop"
  time_start = "23:00"
  time_end   = "07:00"
  days       = ["Friday", "Saturday"]
}

# Allow gaming servers only on weekends
scheduled_rule "weekend-gaming" {
  enabled    = true
  zone_from  = "kids"
  zone_to    = "wan"
  proto      = "udp"
  dest_ports = [3074, 3478, 3479, 3480]
  action     = "accept"
  time_start = "10:00"
  time_end   = "21:00"
  days       = ["Saturday", "Sunday"]
}

# Maintenance window - allow full access for updates (Sunday 3am-5am)
scheduled_rule "maintenance-window" {
  enabled    = true
  zone_from  = "lan"
  zone_to    = "wan"
  action     = "accept"
  time_start = "03:00"
  time_end   = "05:00"
  days       = ["Sunday"]
  comment    = "Unrestricted access for automated updates"
}

# -----------------------------------------------------------------------------
# Regular Policies with Time-Based Rules
# -----------------------------------------------------------------------------

policy "kids" "wan" {
  # Default action when no scheduled rules match
  action = "accept"

  # Always block social media (can be overridden by scheduled_rules above)
  rule "block-social-media" {
    dest_ipset = "social_media_ips"
    action     = "drop"
    log        = true
    log_prefix = "SOCIAL-BLOCK: "
  }

  # Time-based rule within policy (alternative to scheduled_rule)
  rule "homework-hours-allow-educational" {
    dest_ipset = "educational_sites"
    action     = "accept"
    time_start = "15:00"
    time_end   = "18:00"
    days       = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
  }
}

policy "lan" "wan" {
  action = "accept"
}

policy "kids" "self" {
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
}

policy "lan" "self" {
  rule "allow-all" {
    action = "accept"
  }
}

# -----------------------------------------------------------------------------
# IPSets for Content Filtering
# -----------------------------------------------------------------------------

ipset "social_media_ips" {
  description = "Social media platform IPs"
  type        = "ipv4_addr"
  entries     = [
    "157.240.0.0/16",
    "31.13.24.0/21",
  ]
}

ipset "educational_sites" {
  description = "Educational website IPs"
  type        = "dns"
  domains     = [
    "khanacademy.org",
    "wikipedia.org",
    "*.edu",
  ]
  refresh_interval = "1h"
}

# -----------------------------------------------------------------------------
# DHCP for Kids Network
# -----------------------------------------------------------------------------

dhcp {
  enabled = true

  scope "kids-scope" {
    interface   = "eth2"
    range_start = "192.168.50.100"
    range_end   = "192.168.50.200"
    router      = "192.168.50.1"
    dns         = ["192.168.50.1"]

    reservation "AA:BB:CC:DD:EE:01" {
      ip          = "192.168.50.10"
      hostname    = "kids-tablet"
      description = "Kids tablet"
    }

    reservation "AA:BB:CC:DD:EE:02" {
      ip          = "192.168.50.11"
      hostname    = "kids-laptop"
      description = "Kids laptop"
    }
  }
}

# -----------------------------------------------------------------------------
# NAT
# -----------------------------------------------------------------------------

nat "masquerade" {
  type          = "masquerade"
  out_interface = "eth0"
}
