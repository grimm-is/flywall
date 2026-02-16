# IPS-TC Integration Configuration Example
# This example demonstrates how to configure the IPS engine with TC fast-path offload

schema_version = "1.0"

# Enable eBPF features
ebpf {
  enabled = true

  features {
    # TC offload for fast-path processing
    tc_offload {
      enabled = true
      config = {
        max_flows = 100000
        flow_timeout = "5m"
      }
    }

    # IPS integration with TC
    ips_integration {
      enabled = true
      config = {
        # Number of packets to inspect before considering offload
        inspection_window = 10

        # Number of trusted flows before automatic offload
        offload_threshold = 5

        # Maximum number of flows to track
        max_pending_flows = 10000

        # Cleanup interval for expired flows
        cleanup_interval = "5m"

        # Statistics flush interval
        stats_flush_interval = "30s"
      }
    }

    # QoS management
    qos {
      enabled = true
      config = {
        default_profiles = true
      }
    }

    # DNS blocklist (optional)
    dns_blocklist {
      enabled = false
    }
  }
}

# Security configuration with inline IPS
security {
  mode = "inline"

  rules {
    # Allow established connections
    allow {
      description = "Allow established connections"
      state = "established"
    }

    # Allow SSH
    allow {
      description = "Allow SSH"
      protocol = "tcp"
      src_port = 22
      verdict = "accept"
    }

    # Allow HTTP/HTTPS
    allow {
      description = "Allow web traffic"
      protocol = "tcp"
      dst_port = [80, 443]
      verdict = "accept"
    }

    # Default rule - send to learning engine
    learning {
      description = "Learn new traffic patterns"
      verdict = "learn"
      config = {
        packet_window = 10
        offload_mark = 2097152  # 0x200000 in decimal
      }
    }
  }

  learning {
    enabled = true
    mode = "automatic"

    # Learning configuration
    min_packets = 5
    confidence_threshold = 0.95

    # Automatic offloading to TC fast path
    auto_offload = true
    offload_after = 10
  }
}

# Network configuration
network {
  interfaces {
    eth0 {
      type = "physical"
      role = "external"

      # Enable TC offload on this interface
      tc_offload = true

      # Enable IPS processing
      ips_enabled = true
    }

    eth1 {
      type = "physical"
      role = "internal"

      # Enable TC offload on this interface
      tc_offload = true

      # Enable IPS processing
      ips_enabled = true
    }
  }
}

# Logging configuration
logging {
  level = "info"

  outputs {
    file {
      path = "/var/log/flywall.log"
      format = "json"
    }

    syslog {
      facility = "daemon"
      tag = "flywall"
    }
  }
}

# Statistics configuration
statistics {
  enabled = true

  # Export statistics
  exporters {
    prometheus {
      enabled = true
      listen = "0.0.0.0:9090"
      path = "/metrics"
    }
  }

  # Internal statistics
  internal {
    # TC statistics
    tc_stats = true

    # IPS statistics
    ips_stats = true

    # Flow statistics
    flow_stats = true

    # QoS statistics
    qos_stats = true
  }
}
