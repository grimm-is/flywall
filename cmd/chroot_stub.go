// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build !linux

package cmd

func setupChroot(jailPath string) error {
	Printer.Println("Warning: Chroot not supported on this OS")
	return nil
}

func enterChroot(jailPath string) error {
	return nil
}
