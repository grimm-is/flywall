# IPS Verdict Application Configuration Example
# This example demonstrates how to configure IPS verdict application with TC fast-path

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

    # IPS integration with TC, pattern matching, and verdict application
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
        }

        # Verdict application configuration
        verdict_config = {
          enabled = true
          default_action = "allow"
          hard_fail_on_errors = false
          verdict_timeout = "100ms"
          max_pending_verdicts = 1000
          bypass_on_overload = true
          log_verdicts = true
          log_packet_samples = false
          sample_rate = 0.01
        }

        # Verdict cache configuration
        verdict_cache_size = 10000
        verdict_cache_ttl = "5m"
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

# Security configuration with IPS verdict application
security {
  mode = "inline"

  rules {
    # Allow established connections
    allow {
      description = "Allow established connections"
      state = "established"
      verdict = "accept"
    }

    # SSH with monitoring
    allow {
      description = "Allow SSH with monitoring"
      protocol = "tcp"
      src_port = 22
      verdict = "accept"
      config = {
        monitor = true
        verdict_application = {
          action = "monitor"
          reason = "SSH connections monitored for bruteforce"
        }
      }
    }

    # HTTP/HTTPS with deep inspection
    allow {
      description = "Allow web traffic with deep inspection"
      protocol = "tcp"
      dst_port = [80, 443]
      verdict = "accept"
      config = {
        deep_inspection = true
        verdict_application = {
          action = "allow"
          offload = true
          reason = "Web traffic allowed and offloaded"
        }
      }
    }

    # Block known malicious traffic
    block {
      description = "Block known malicious traffic"
      verdict = "drop"
      config = {
        verdict_application = {
          action = "drop"
          reason = "Malicious traffic blocked by policy"
        }
      }
    }

    # Default rule - send to learning engine with full IPS
    learning {
      description = "Learn new traffic patterns with full IPS"
      verdict = "learn"
      config = {
        packet_window = 10
        offload_mark = 2097152  # 0x200000 in decimal
        pattern_matching = true
        verdict_application = {
          action = "inspect"
          reason = "New flow undergoing IPS inspection"
        }
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

      # Traffic inspection settings
      inspection {
        deep_packet_inspection = true
        extract_payloads = true
        max_payload_size = 1500
      }

      # Verdict application settings
      verdict_application {
        enabled = true
        default_action = "allow"
        cache_enabled = true
        cache_size = 10000
        cache_ttl = "5m"
        log_all_verdicts = false
        log_samples = true
        sample_rate = 0.001
      }
    }

    eth1 {
      type = "physical"
      role = "internal"

      # Enable TC offload on this interface
      tc_offload = true

      # Enable IPS processing
      ips_enabled = true

      # Less strict inspection for internal traffic
      inspection {
        deep_packet_inspection = false
        extract_payloads = false
      }

      # Verdict application settings
      verdict_application {
        enabled = true
        default_action = "allow"
        cache_enabled = true
        cache_size = 5000
        cache_ttl = "10m"
        log_all_verdicts = false
        log_samples = false
      }
    }
  }
}

# Verdict application rules
verdict_application {
  # Verdict priorities (higher takes precedence)
  priorities = {
    "drop" = 100
    "monitor" = 50
    "allow" = 10
    "offload" = 20
    "bypass" = 5
  }

  # Verdict timeouts
  timeouts = {
    "drop" = "1h"
    "monitor" = "30m"
    "allow" = "24h"
    "offload" = "1h"
  }

  # Verict conditions
  conditions {
    # High severity patterns -> drop
    high_severity {
      severity_min = 7
      action = "drop"
      reason = "High severity pattern detected"
    }

    # Medium severity patterns -> monitor
    medium_severity {
      severity_min = 4
      severity_max = 6
      action = "monitor"
      reason = "Medium severity pattern detected"
    }

    # Known trusted flows -> offload
    trusted_flows {
      packet_count_min = 100
      no_violations = true
      action = "offload"
      reason = "Trusted flow offloaded to kernel"
    }

    # Suspicious flows -> monitor
    suspicious_flows {
      packet_count_min = 10
      violations_max = 1
      action = "monitor"
      reason = "Suspicious flow monitoring"
    }
  }

  # Verict overrides
  overrides {
    # Override drop to allow for specific sources
    allow_sources {
      sources = ["192.168.1.0/24", "10.0.0.0/8"]
      original_action = "drop"
      new_action = "allow"
      reason = "Internal network exception"
    }

    # Override allow to monitor for specific destinations
    monitor_destinations {
      destinations = ["203.0.113.0/24"]
      original_action = "allow"
      new_action = "monitor"
      reason = "Monitor traffic to suspicious network"
    }
  }

  # Verdict escalation
  escalation {
    enabled = true

    # Escalation rules
    rules {
      monitor_to_drop {
        current_action = "monitor"
        violations_threshold = 5
        time_window = "10m"
        new_action = "drop"
        reason = "Escalated from monitor to drop due to violations"
      }

      allow_to_monitor {
        current_action = "allow"
        violations_threshold = 3
        time_window = "5m"
        new_action = "monitor"
        reason = "Escalated from allow to monitor due to violations"
      }
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

  # Verdict application logging
  verdict_application {
    log_all_verdicts = false
    log_drops = true
    log_monitors = true
    log_offloads = true
    log_samples = true
    sample_rate = 0.001

    # Include in logs
    include_flow_key = true
    include_verdict_reason = true
    include_pattern_matches = true
    include_processing_time = true
    include_cache_hit = true
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

    # Verdict application statistics
    verdict_stats = true

    # Flow statistics
    flow_stats = true

    # QoS statistics
    qos_stats = true

    # Cache statistics
    cache_stats = true
  }
}
