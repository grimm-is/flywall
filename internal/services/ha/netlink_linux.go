// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build linux
// +build linux

package ha

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// parseAddr parses an IP address in CIDR notation.
func parseAddr(addrStr string) (*netlink.Addr, error) {
	ip, ipNet, err := net.ParseCIDR(addrStr)
	if err != nil {
		return nil, err
	}
	ipNet.IP = ip
	return &netlink.Addr{IPNet: ipNet}, nil
}

// addIPAddress adds an IP address to an interface.
func addIPAddress(ifaceName string, addr *netlink.Addr, label string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	if label != "" {
		addr.Label = label
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		// Check if already exists
		if err.Error() == "file exists" {
			return nil
		}
		return fmt.Errorf("failed to add address to %s: %w", ifaceName, err)
	}

	return nil
}

// removeIPAddress removes an IP address from an interface.
func removeIPAddress(ifaceName string, addr *netlink.Addr) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	if err := netlink.AddrDel(link, addr); err != nil {
		// Ignore "not found" errors
		if err.Error() == "no such process" {
			return nil
		}
		return fmt.Errorf("failed to remove address from %s: %w", ifaceName, err)
	}

	return nil
}

// hasIPAddress checks if an interface has a specific IP address.
func hasIPAddress(ifaceName string, targetIP net.IP) (bool, error) {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return false, fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return false, fmt.Errorf("failed to list addresses on %s: %w", ifaceName, err)
	}

	for _, addr := range addrs {
		if addr.IP.Equal(targetIP) {
			return true, nil
		}
	}

	return false, nil
}
