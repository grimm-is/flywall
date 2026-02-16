// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux

package cmd

func setupNetworkNamespace() error {
	return nil
}

func isIsolated() bool {
	return false
}

func configureHostFirewall(interfaces []string) error {
	return nil
}

func configureHostFirewallWithNetworks(networks []string) error {
	return nil
}
