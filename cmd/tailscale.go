// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"grimm.is/flywall/internal/tailscale"
)

// RunTailscale implements the 'fw tailscale' command
func RunTailscale(args []string) error {
	if len(args) < 1 {
		printTailscaleUsage()
		return nil
	}

	command := args[0]
	// shift args
	subArgs := args[1:]

	switch command {
	case "install":
		return runTailscaleInstall(subArgs)
	case "status":
		return runTailscaleStatus(subArgs)
	case "up":
		return runTailscaleProxyCmd("up", subArgs)
	case "down":
		return runTailscaleProxyCmd("down", subArgs)
	case "login":
		return runTailscaleProxyCmd("login", subArgs)
	case "help":
		printTailscaleUsage()
		return nil
	default:
		// If unknown, maybe just pass through to tailscale CLI?
		// For now, be strict.
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printTailscaleUsage() {
	fmt.Println("Usage: flywall tailscale <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install    Download and install the latest Tailscale release")
	fmt.Println("  status     Show Tailscale status (via API)")
	fmt.Println("  up         Bring Tailscale up (passes args to tailscale CLI)")
	fmt.Println("  down       Bring Tailscale down (passes args to tailscale CLI)")
	fmt.Println("  login      Log in to Tailscale (passes args to tailscale CLI)")
}

func runTailscaleInstall(args []string) error {
	// Simple wrapper for now
	return tailscale.Install()
}

func runTailscaleStatus(args []string) error {
	// specific flags for status?
	jsonOutput := false
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.BoolVar(&jsonOutput, "json", false, "Output JSON")
	fs.Parse(args)

	client := tailscale.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal status to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Tailscale Status\n")
	fmt.Printf("Backend State: %s\n", status.BackendState)
	fmt.Printf("Version: %s\n", status.Version)
	fmt.Printf("Auth URL: %s\n", status.AuthURL)
	if status.Self != nil {
		fmt.Printf("Self: %v (%v)\n", status.Self.DNSName, status.Self.TailscaleIPs)
	}

	if len(status.Peer) > 0 {
		fmt.Printf("\nPeers:\n")
		for _, peer := range status.Peer {
			online := "offline"
			if peer.Online {
				online = "online"
			}
			fmt.Printf("  %-20s %-15v %s\n", peer.HostName, peer.TailscaleIPs, online)
		}
	} else {
		fmt.Printf("\nNo peers found.\n")
	}

	return nil
}

// runTailscaleProxyCmd passes commands through to the system 'tailscale' binary
// This is useful for interactive commands like 'up' and 'login'
func runTailscaleProxyCmd(subcmd string, args []string) error {
	binPath := "/usr/local/bin/tailscale"
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return fmt.Errorf("tailscale binary not found at %s. Run 'flywall tailscale install' first", binPath)
	}

	cmdArgs := append([]string{subcmd}, args...)
	cmd := exec.Command(binPath, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
