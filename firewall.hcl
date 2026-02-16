# Demo Firewall Configuration
# Basic setup for demonstration purposes

ip_forwarding = true


# Interface configuration
interface "eth0" {
  description   = "WAN Interface"
  dhcp          = true
  access_web_ui = true
}

interface "eth1" {
  description = "Green Zone (Trusted LAN)"
  ipv4        = ["10.1.0.1/24"]
  dhcp        = false
}




zone "mgmt" {
  description = "Trusted Management Zone"
  management {
    web_ui = true
    api    = true
    ssh    = true
    icmp   = true
    web    = false
    snmp   = false
    syslog = false
  }
}




zone "WAN" {
}
zone "Green" {
}
zone "Orange" {
}
zone "Red" {
}
api {
  enabled         = true
  listen          = "0.0.0.0:8080"
  disable_sandbox = true
}
system {
  sysctl_profile = "default"
}
policy "Green" "WAN" {
  name = "green_to_wan"
  rule "allow_internet" {
    description = "Allow Green zone internet access"
    action      = "accept"
  }
}
policy "Orange" "WAN" {
  name = "orange_to_wan"
  rule "allow_internet" {
    description = "Allow Orange zone internet access"
    action      = "accept"
  }
}
policy "Red" "WAN" {
  name = "red_blocked"
  rule "block_internet" {
    description = "Block Red zone from internet"
    action      = "drop"
  }
}
nat "outbound" {
  type          = "masquerade"
  out_interface = "eth0"
}
mdns {
  enabled    = true
  interfaces = ["eth0", "eth1", "eth2", "eth3", "eth4"]
}
