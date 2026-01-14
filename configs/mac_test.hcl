schema_version      = "1.0"
ip_forwarding       = true
mss_clamping        = true
enable_flow_offload = true


zone "wan" {
}

zone "lan" {
}

zone "opt" {
}

interface "lo" {
  ipv4 = ["127.0.0.1/8"]
  dhcp = false
}

interface "eth0" {
  dhcp = true
}

interface "eth1" {
  ipv4 = ["192.168.1.1/24"]
  dhcp = false
}

interface "eth5" {
  disabled = true
  dhcp     = false
}

system {
  sysctl_profile = "default"
}
dns {
  mode = "forward"
}
mdns {
  enabled    = true
  interfaces = ["eth0", "eth1", "eth5"]
}
