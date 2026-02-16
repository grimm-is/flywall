# QoS Configuration Example
# This example demonstrates how to configure Quality of Service (QoS) profiles
# and apply them to different types of traffic.

schema_version = "1.0"

# Enable eBPF features including TC offload
ebpf {
  enabled = true

  features {
    tc_offload {
      enabled = true
      config = {
        # TC offload configuration
        max_flows = 100000
        flow_timeout = "5m"
      }
    }

    dns_blocklist {
      enabled = false
    }
  }
}

# Security configuration with learning
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
        offload_mark = 0x200000
      }
    }
  }

  learning {
    enabled = true
    mode = "automatic"

    # Learning configuration
    min_packets = 5
    confidence_threshold = 0.95

    # Automatic offloading
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

      # Default QoS profile for unknown traffic
      default_qos_profile = "bulk"
    }

    eth1 {
      type = "physical"
      role = "internal"

      # Enable TC offload on this interface
      tc_offload = true

      # Default QoS profile for internal traffic
      default_qos_profile = "interactive"
    }
  }
}

# QoS configuration
qos {
  # Default QoS profiles
  profiles {
    default {
      rate_limit = 0        # Unlimited
      burst_limit = 0
      priority = 0
      app_class = 0
    }

    bulk {
      rate_limit = "10Mbps"  # 10 megabits per second
      burst_limit = "1MB"
      priority = 1
      app_class = 1
    }

    interactive {
      rate_limit = "5Mbps"   # 5 megabits per second
      burst_limit = "500KB"
      priority = 3
      app_class = 2
    }

    video {
      rate_limit = "20Mbps"  # 20 megabits per second
      burst_limit = "2MB"
      priority = 4
      app_class = 3
    }

    voice {
      rate_limit = "1Mbps"   # 1 megabit per second
      burst_limit = "100KB"
      priority = 5
      app_class = 4
    }

    critical {
      rate_limit = 0        # Unlimited
      burst_limit = 0
      priority = 7
      app_class = 5
    }
  }

  # Traffic classification rules
  rules {
    # Video streaming traffic
    video {
      description = "Video streaming services"
      dst_ports = [1935, 554, 8080]
      protocols = ["tcp", "udp"]
      qos_profile = "video"

      # Domain-based classification
      domains = [
        "youtube.com",
        "netflix.com",
        "twitch.tv",
        "vimeo.com"
      ]
    }

    # VoIP traffic
    voice {
      description = "Voice over IP"
      dst_ports = [5060, 5061, 16384-32768]
      protocols = ["udp"]
      qos_profile = "voice"

      # Domain-based classification
      domains = [
        "sip.example.com",
        "voip.example.com"
      ]
    }

    # Interactive traffic (SSH, RDP, etc.)
    interactive {
      description = "Interactive sessions"
      dst_ports = [22, 3389, 5900]
      protocols = ["tcp"]
      qos_profile = "interactive"
    }

    # Bulk transfers (FTP, SCP, etc.)
    bulk {
      description = "Bulk data transfers"
      dst_ports = [20, 21, 22]
      protocols = ["tcp"]
      qos_profile = "bulk"
    }

    # Critical services
    critical {
      description = "Critical infrastructure"
      src_networks = ["10.0.0.0/8", "192.168.0.0/16"]
      dst_ports = [53, 123, 443]
      protocols = ["tcp", "udp"]
      qos_profile = "critical"
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
