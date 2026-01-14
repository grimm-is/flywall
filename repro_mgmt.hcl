schema_version = "1.0"
ip_forwarding = true

zone "LAN" {
    description = "Default LAN"
    action = "drop"
    management {
        ssh = true
        web = false
    }
}

zone "GUEST" {
    description = "Guest Network"
    action = "drop"
    management {
        ssh = false
        web = false
    }
}

interface "veth_eth1" {
    description = "LAN Interface (Use Zone Default)"
    zone        = "LAN"
    ipv4        = ["10.1.0.1/24"]
}

interface "veth_eth2" {
    description = "Admin Interface (Override Zone)"
    zone        = "LAN"
    ipv4        = ["10.2.0.1/24"]
    management {
        ssh = true
        web = false
        api = true
    }
}

interface "veth_eth3" {
    description = "Guest Interface (Implicit Zone Test)"
    # No Zone defined! Implicit zone "veth_eth3"
    ipv4        = ["10.3.0.1/24"]
}

policy "LAN" "WAN" {
    name = "lan_to_wan"
    rule "allow_icmp" {
        proto  = "icmp"
        action = "accept"
    }
}

policy "veth_eth3" "WAN" {
    name = "eth3_to_wan"
    rule "allow_icmp_eth3" {
        proto  = "icmp"
        action = "accept"
    }
}

interface "veth_eth0" {
    description = "WAN"
    zone        = "WAN"
    ipv4        = ["10.0.0.1/24"]
}
