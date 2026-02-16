#!/bin/sh
# Fix all configs in 07-fallback-mechanisms.sh

BOILERPLATE='schema_version = "1.0"
ip_forwarding = true

interface "eth0" {
    zone = "wan"
    ipv4 = ["10.0.2.15/24"]
    gateway = "10.0.2.2"
}

zone "wan" {
    interfaces = ["eth0"]
}

api {
    enabled = true
    listen = "0.0.0.0:8080"
}

logging {
    level = "info"
    file = "/tmp/flywall-test.log"
}

control_plane {
    enabled = true
    socket = "/tmp/flywall.ctl"
}'

# Create a temporary file with the corrected content
cp 07-fallback-mechanisms.sh 07-fallback-mechanisms.sh.bak

# Process the file to add boilerplate to each config section
awk '
BEGIN { in_config = 0; skip_next = 0 }
/^# Test/ {
    if (in_config) {
        print "EOF"
        in_config = 0
    }
    print
    next
}
/cat > "\$CONFIG_FILE" <<.EOF./ {
    in_config = 1
    print $0
    print "'"$BOILERPLATE"'"
    skip_next = 2
    next
}
/^schema_version = "1.0"$/ && in_config && skip_next > 0 {
    skip_next--
    next
}
/^ip_forwarding = true$/ && in_config && skip_next > 0 {
    skip_next--
    next
}
/^}$/ && in_config && /config/ {
    print "EOF"
    in_config = 0
    print
    next
}
/^EOF$/ && in_config {
    in_config = 0
    print
    next
}
{
    if (!in_config || skip_next == 0) print
}
' 07-fallback-mechanisms.sh.bak > 07-fallback-mechanisms.sh

echo "Fixed fallback mechanisms test"
