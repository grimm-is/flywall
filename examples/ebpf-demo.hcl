# eBPF Demo Configuration
# This demonstrates the eBPF features of Flywall

schema_version = "1.0"

# Basic network configuration
ip_forwarding = true

interface "eth0" {
    zone = "wan"
    ipv4 = ["10.0.2.15/24"]
    gateway = "10.0.2.2"
}

interface "eth1" {
    zone = "lan"
    ipv4 = ["192.168.1.1/24"]
}

zone "wan" {
    interfaces = ["eth0"]
}

zone "lan" {
    interfaces = ["eth1"]
}

# API for monitoring and control
api {
    enabled = true
    listen = "0.0.0.0:8080"
    require_auth = false
}

# eBPF Configuration
ebpf {
  # Enable eBPF subsystem
  enabled = true

  # Feature configuration
  feature "ddos_protection" {
    enabled = true
    priority = 100
    config = {
      # Blocklist settings
      max_blocked_ips = 1000000
      block_duration = "24h"

      # Rate limiting
      rate_limit_pps = 10000
      rate_limit_burst = 1000

      # Thresholds
      connection_threshold = 1000
      packet_threshold = 10000
    }
  }

  feature "dns_blocklist" {
    enabled = true
    priority = 95
    config = {
      # Bloom filter settings
      bloom_size = 131072  # 1MB
      hash_count = 7

      # Blocklist sources
      sources = [
        "https://blocklist.example.com/dns.txt",
        "/etc/flywall/dns_blocklist.txt"
      ]

      # Update interval
      update_interval = "1h"
    }
  }

  feature "flow_monitoring" {
    enabled = true
    priority = 85
    config = {
      # Flow table settings
      max_flows = 1000000
      flow_timeout = "5m"

      # Statistics
      enable_per_flow_stats = true
      stats_interval = "1s"
    }
  }

  feature "inline_ips" {
    enabled = true
    priority = 90
    config = {
      # IPS settings
      mode = "alert"  # alert, block, or both

      # Pattern matching
      max_patterns = 10000
      pattern_update_interval = "5m"

      # Learning
      learning_enabled = true
      learning_threshold = 0.95
    }
  }

  feature "qos" {
    enabled = true
    priority = 60
    config = {
      # QoS profiles
      profiles = {
        "high" = {
          bandwidth = "100Mbps"
          priority = 100
        }
        "low" = {
          bandwidth = "1Mbps"
          priority = 10
        }
      }

      # Default profile
      default_profile = "high"
    }
  }

  feature "tls_fingerprinting" {
    enabled = true
    priority = 70
    config = {
      # JA3 settings
      enable_ja3 = true
      ja3_threshold = 0.8

      # SNI extraction
      enable_sni = true
      sni_filter = [
        "malware.example.com",
        "phishing.example.com"
      ]
    }
  }

  feature "device_discovery" {
    enabled = true
    priority = 40
    config = {
      # DHCP monitoring
      enable_dhcp = true

      # ARP tracking
      enable_arp = true

      # Device fingerprinting
      enable_fingerprinting = true
    }
  }

  feature "statistics" {
    enabled = true
    priority = 20
    config = {
      # Export settings
      export_interval = "1s"

      # Metrics
      enable_prometheus = true
      prometheus_port = 9090

      # Events
      enable_events = true
      event_buffer_size = 10000
    }
  }

  # Performance settings
  performance {
    max_cpu_percent = 80
    max_memory_mb = 500
    max_events_per_sec = 10000
    max_pps = 10000000
  }

  # Adaptive performance management
  adaptive {
    enabled = true
    scale_back_threshold = 80
    scale_back_rate = 0.1
    minimum_features = ["ddos_protection", "flow_monitoring"]

    sampling {
      enabled = true
      min_sample_rate = 0.1
      max_sample_rate = 1.0
      adaptive_rate = true
    }
  }

  # Map configuration
  maps {
    max_maps = 100
    max_map_entries = 1000000
    max_map_memory = 100
    cache_size = 1000
  }

  # Program configuration
  programs {
    xdp_blocklist = "xdp_blocklist.o"
    tc_classifier = "tc_classifier.o"
    socket_dns = "socket_dns.o"
    socket_tls = "socket_tls.o"
    socket_dhcp = "socket_dhcp.o"
  }

  # Fallback configuration
  fallback {
    enable_nfqueue = true
    partial_support = true
    on_load_failure = "disable_feature"
    on_map_failure = "reduce_capacity"
    on_hook_failure = "try_alternative"
    on_verifier_failure = "use_simpler_version"
    recovery_interval = "30s"
  }
}

# Example policy using eBPF features
policy "wan" "lan" {
  description = "WAN to LAN traffic with eBPF protection"

  # Default action
  action = "accept"
}
