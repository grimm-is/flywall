# Common eBPF test configuration boilerplate
# This prevents safemode activation

schema_version = "1.0"
ip_forwarding = true

# Network configuration for test environment
interface "eth0" {
    zone = "wan"
    ipv4 = ["10.0.2.15/24"]
    gateway = "10.0.2.2"
}

zone "wan" {
    match {
        interface = "eth0"
    }
}

# API for testing
api {
    enabled = true
    listen = "0.0.0.0:8080"
}

# Logging
logging {
    level = "info"
    file = "/tmp/flywall-test.log"
}

# Control plane
control_plane {
    enabled = true
    socket = "/tmp/flywall.ctl"
}
