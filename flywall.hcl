api {
    enabled = true
    require_auth = false
}

web {
    listen = ":9090"
    serve_ui = true

    # Allow from localhost
    allow {
        interfaces = ["lo"]
        sources = ["127.0.0.1"]
    }

    # Deny specific test IP
    deny {
        sources = ["1.2.3.4"]
    }
}

interface "lo" {
    # Legacy flag should be ignored in favor of web block?
    # Or additive. But here we test explicit web block.
    # ipv4 = ... (default lo has verified address usually)
}

interface "eth0" {
    # ipv4 = ["10.0.2.15/24"] # Normally managed by DHCP
}
