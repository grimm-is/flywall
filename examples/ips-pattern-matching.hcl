# IPS Pattern Matching Configuration Example
# This example demonstrates how to configure IPS pattern matching with TC fast-path

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

    # IPS integration with TC and pattern matching
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

        # Pattern matching configuration
        pattern_config = {
          enabled = true
          max_signatures = 10000
          max_domain_rules = 5000
          max_ip_rules = 10000
          regex_cache_size = 1000
          match_timeout = "10ms"
          update_interval = "1h"
        }

        # Pattern database configuration
        pattern_db_config = {
          enabled = true
          local_db_path = "/etc/flywall/patterns.db"
          remote_url = "https://updates.flywall.io/patterns"
          update_interval = "24h"
          update_timeout = "30s"
          auto_update = true
          signature_sources = [
            "https://rules.emergingthreats.net/open/suricata.rules",
            "https://github.com/Neo23x0/signature-base/raw/master/"
          ]
        }
      }
    }

    # QoS management
    qos {
      enabled = true
      config = {
        default_profiles = true
      }
    }
  }
}

# Security configuration with IPS and pattern matching
security {
  mode = "inline"

  rules {
    # Allow established connections
    allow {
      description = "Allow established connections"
      state = "established"
    }

    # Allow SSH (but monitor for bruteforce)
    allow {
      description = "Allow SSH with monitoring"
      protocol = "tcp"
      src_port = 22
      verdict = "accept"
      config = {
        monitor = true
      }
    }

    # Allow HTTP/HTTPS (with pattern inspection)
    allow {
      description = "Allow web traffic with inspection"
      protocol = "tcp"
      dst_port = [80, 443]
      verdict = "accept"
      config = {
        deep_inspection = true
      }
    }

    # Default rule - send to learning engine with pattern matching
    learning {
      description = "Learn new traffic patterns with IPS inspection"
      verdict = "learn"
      config = {
        packet_window = 10
        offload_mark = 2097152  # 0x200000 in decimal
        pattern_matching = true
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

      # Enable IPS processing with pattern matching
      ips_enabled = true
      pattern_matching = true

      # Traffic inspection settings
      inspection {
        deep_packet_inspection = true
        extract_payloads = true
        max_payload_size = 1500
      }
    }

    eth1 {
      type = "physical"
      role = "internal"

      # Enable TC offload on this interface
      tc_offload = true

      # Enable IPS processing with pattern matching
      ips_enabled = true
      pattern_matching = true

      # Less strict inspection for internal traffic
      inspection {
        deep_packet_inspection = false
        extract_payloads = false
      }
    }
  }
}

# Pattern matching rules
pattern_matching {
  # Signature categories
  categories {
    malware {
      enabled = true
      action = "block"
      severity_threshold = 5
    }

    trojan {
      enabled = true
      action = "block"
      severity_threshold = 6
    }

    webshell {
      enabled = true
      action = "block"
      severity_threshold = 4
    }

    scanner {
      enabled = true
      action = "monitor"
      severity_threshold = 3
    }

    policy {
      enabled = true
      action = "allow"
      severity_threshold = 2
    }
  }

  # Custom signatures
  signatures {
    # Block suspicious PowerShell commands
    powershell_encoded {
      pattern = "(?i)powershell.*-enc.*[A-Za-z0-9+/]+={0,2}"
      type = "regex"
      severity = 5
      category = "malware"
      enabled = true
    }

    # Detect web shells
    webshell_php {
      pattern = "eval(base64_decode"
      type = "literal"
      severity = 7
      category = "webshell"
      enabled = true
    }

    # Monitor Nmap scans
    nmap_scan {
      pattern = "User-Agent:.*NSE"
      type = "regex"
      severity = 2
      category = "scanner"
      enabled = true
    }
  }

  # Domain rules
  domain_rules {
    # Block known malicious domains
    block_malicious {
      domains = ["*.malware.example.com", "phishing-site.example.org"]
      action = "block"
      severity = 8
      ttl = "24h"
    }

    # Monitor suspicious domains
    monitor_suspicious {
      domains = ["suspicious-domain.net"]
      action = "monitor"
      severity = 5
      ttl = "12h"
    }
  }

  # IP rules
  ip_rules {
    # Block known malicious IP ranges
    block_malicious_ips {
      networks = ["192.0.2.0/24", "203.0.113.0/24"]
      action = "block"
      severity = 7
      ttl = "168h"
    }

    # Monitor suspicious IPs
    monitor_suspicious_ips {
      networks = ["198.51.100.0/24"]
      action = "monitor"
      severity = 4
      ttl = "24h"
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

  # Pattern matching logs
  pattern_matching {
    log_matches = true
    log_level = "info"
    include_payload = false
    max_payload_log = 256
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

    # Pattern matching statistics
    pattern_stats = true

    # Flow statistics
    flow_stats = true

    # QoS statistics
    qos_stats = true
  }
}
