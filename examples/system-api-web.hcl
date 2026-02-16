# System, API, and Web UI Configuration
#
# This example demonstrates:
# 1. System tuning settings
# 2. API server configuration
# 3. Web UI configuration
# 4. Feature flags
# 5. State replication (HA)

schema_version = "1.0"
ip_forwarding = true

# State directory override (default: /var/lib/flywall)
state_dir = "/opt/flywall/state"

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

  # Per-interface management settings
  management {
    web  = true
    ssh  = true
    api  = true
    icmp = true
  }
}

interface "eth2" {
  description = "Management VLAN"
  zone        = "mgmt"
  ipv4        = ["10.0.0.1/24"]

  management {
    web  = true
    ssh  = true
    api  = true
    icmp = true
    snmp = true
  }
}

# -----------------------------------------------------------------------------
# System Tuning
# -----------------------------------------------------------------------------

system {
  # Hostname
  hostname = "fw01.example.com"

  # Kernel tuning
  conntrack_max       = 262144
  conntrack_timeout   = 600
  somaxconn           = 4096
  tcp_fin_timeout     = 30
  tcp_keepalive_time  = 600
  tcp_keepalive_intvl = 60
  tcp_keepalive_probes = 3

  # Resource limits
  max_open_files = 65535
  core_pattern   = "/var/crash/core.%e.%p"

  # Timezone
  timezone = "UTC"

  # Auto-updates
  auto_update_enabled  = true
  auto_update_schedule = "0 3 * * 0"
}

# -----------------------------------------------------------------------------
# API Server Configuration
# -----------------------------------------------------------------------------

api {
  enabled     = true
  listen_addr = "0.0.0.0:8443"

  # TLS configuration
  tls_cert_file = "/etc/flywall/certs/api.crt"
  tls_key_file  = "/etc/flywall/certs/api.key"

  # Authentication
  auth_type = "token"

  # API tokens (hashed)
  tokens = [
    "admin:$argon2id$v=19$m=65536,t=3,p=4$...",
    "readonly:$argon2id$v=19$m=65536,t=3,p=4$..."
  ]

  # Rate limiting
  rate_limit_enabled = true
  rate_limit_rps     = 100
  rate_limit_burst   = 200

  # CORS (for web UI on different origin)
  cors_origins = ["https://admin.example.com"]

  # Request logging
  access_log = "/var/log/flywall/api-access.log"
}

# -----------------------------------------------------------------------------
# Web UI Configuration
# -----------------------------------------------------------------------------

web {
  enabled     = true
  listen_addr = "0.0.0.0:443"

  # TLS
  tls_cert_file = "/etc/flywall/certs/web.crt"
  tls_key_file  = "/etc/flywall/certs/web.key"

  # Let's Encrypt automatic certificates
  # letsencrypt_enabled = true
  # letsencrypt_email   = "admin@example.com"
  # letsencrypt_domains = ["firewall.example.com"]

  # HTTP redirect to HTTPS
  http_redirect = true
  http_port     = 80

  # Session settings
  session_timeout  = "24h"
  session_secure   = true
  session_samesite = "strict"

  # Branding
  title = "Flywall Firewall"

  # Access control
  allowed_networks = ["192.168.1.0/24", "10.0.0.0/24"]
}

# -----------------------------------------------------------------------------
# Feature Flags
# -----------------------------------------------------------------------------

features {
  # Enable experimental features
  experimental_nftables_flowtable = true
  experimental_ebpf_offload       = false

  # Performance tuning
  conntrack_helper_ftp  = true
  conntrack_helper_sip  = false
  conntrack_helper_tftp = true

  # Logging features
  per_rule_counters = true
  flow_logging      = true

  # Security features
  strict_reverse_path = true
  drop_invalid_state  = true
}

# -----------------------------------------------------------------------------
# State Replication (High Availability)
# -----------------------------------------------------------------------------

replication {
  enabled = true
  mode    = "active-passive"

  # Cluster peers
  peers = [
    "10.0.0.2:7946",
    "10.0.0.3:7946"
  ]

  # This node's bind address
  bind_addr = "10.0.0.1:7946"

  # Encryption key for cluster traffic
  encrypt_key = "${CLUSTER_ENCRYPT_KEY}"

  # State sync settings
  sync_interval  = "1s"
  sync_timeout   = "5s"

  # Conntrack sync
  conntrack_sync = true

  # Virtual IP (VRRP-style)
  virtual_ip = "192.168.1.254/24"
  vip_interface = "eth1"

  # Failover settings
  failover_timeout = "10s"
  preempt          = true
  priority         = 100
}

# -----------------------------------------------------------------------------
# Zones
# -----------------------------------------------------------------------------

zone "wan" {
  description = "Internet"
  external    = true
}

zone "lan" {
  description = "Local Network"
}

zone "mgmt" {
  description = "Management Network"
}

# -----------------------------------------------------------------------------
# Firewall Policies
# -----------------------------------------------------------------------------

policy "lan" "wan" {
  action = "accept"
}

policy "mgmt" "wan" {
  action = "accept"
}

policy "mgmt" "lan" {
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
}

policy "mgmt" "self" {
  rule "allow-all" {
    action = "accept"
  }
}

# -----------------------------------------------------------------------------
# NAT
# -----------------------------------------------------------------------------

nat "masquerade" {
  type          = "masquerade"
  out_interface = "eth0"
}
