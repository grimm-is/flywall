// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dhcp

import "net"

func parseIPs(ips []string) []net.IP {
	var ret []net.IP
	for _, s := range ips {
		if ip := net.ParseIP(s); ip != nil {
			ret = append(ret, ip)
		}
	}
	return ret
}
