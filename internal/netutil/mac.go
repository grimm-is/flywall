// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package netutil

import (
	"fmt"
	"net"
)

func ParseMAC(macStr string) ([]byte, error) {
	hw, err := net.ParseMAC(macStr)
	if err != nil {
		return nil, err
	}
	return hw, nil
}

func FormatMAC(mac []byte) string {
	if len(mac) != 6 {
		return ""
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

// GenerateVirtualMAC generates a deterministic locally-administered Unicast MAC address
// based on the interface name.
// Prefix: 02:67:63 (Locally Administered, 'g', 'c')
func GenerateVirtualMAC(ifaceName string) []byte {
	hash := uint32(0)
	for _, c := range ifaceName {
		hash = hash*31 + uint32(c)
	}
	return []byte{
		0x02, // Locally-administered, unicast
		0x67, // 'g'
		0x63, // 'c'
		byte(hash >> 16),
		byte(hash >> 8),
		byte(hash),
	}
}
