// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package main

import (
	"os"
	"path/filepath"
	"strings"

	"grimm.is/flywall/tools/pkg/i18n"
	"grimm.is/flywall/tools/pkg/toolbox/agent"
	"grimm.is/flywall/tools/pkg/toolbox/curl"
	"grimm.is/flywall/tools/pkg/toolbox/dhcp"
	"grimm.is/flywall/tools/pkg/toolbox/dig"
	"grimm.is/flywall/tools/pkg/toolbox/harness"
	http "grimm.is/flywall/tools/pkg/toolbox/http"
	"grimm.is/flywall/tools/pkg/toolbox/jq"
	"grimm.is/flywall/tools/pkg/toolbox/mcast"
	"grimm.is/flywall/tools/pkg/toolbox/mdns"
	"grimm.is/flywall/tools/pkg/toolbox/nc"
	"grimm.is/flywall/tools/pkg/toolbox/orca"
	"grimm.is/flywall/tools/pkg/toolbox/sleep"
)

var Printer = i18n.NewCLIPrinter()

func main() {
	if os.Getenv("DEBUG") != "" {
		Printer.Fprintf(os.Stderr, "DEBUG: Toolbox starting, args: %v\n", os.Args)
	}
	// ...
	// Busybox-style dispatch based on argv[0]
	cmd := filepath.Base(os.Args[0])

	// Subcommand mode (toolbox <cmd>)
	if len(os.Args) > 1 && (cmd == "toolbox" || strings.Contains(cmd, "toolbox-")) {
		sub := os.Args[1]
		args := os.Args[2:]
		dispatch(sub, args)
		return
	}

	// Direct invocation (symlinks: "agent", "orca", etc)
	switch cmd {
	case "agent", "orca-agent", "flywall-agent":
		if err := agent.Run(os.Args[1:]); err != nil {
			Printer.Fprintf(os.Stderr, "agent error: %v\n", err)
			os.Exit(1)
		}
	case "orca", "orchestrator", "flywall-orchestrator", "orch", "ctl", "fleet":
		if err := orca.Run(os.Args[1:]); err != nil {
			Printer.Fprintf(os.Stderr, "orca error: %v\n", err)
			os.Exit(1)
		}
	case "prove":
		if err := harness.Run(os.Args[1:]); err != nil {
			Printer.Fprintf(os.Stderr, "prove error: %v\n", err)
			os.Exit(1)
		}
	default:
		// Fallback: If unknown binary name, assume it's toolbox if it has args?
		// Or just print help.
		if len(os.Args) > 1 {
			// Try to handle as subcommand anyway
			dispatch(os.Args[1], os.Args[2:])
			return
		}
		help()
		os.Exit(1)
	}
}

func dispatch(sub string, args []string) {
	switch sub {
	case "agent":
		if err := agent.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "agent error: %v\n", err)
			os.Exit(1)
		}
	case "orca", "orchestrator", "orch", "ctl", "fleet":
		if err := orca.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "orca error: %v\n", err)
			os.Exit(1)
		}
	case "prove":
		if err := harness.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "prove error: %v\n", err)
			os.Exit(1)
		}
	case "mcast":
		if err := mcast.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "mcast error: %v\n", err)
			os.Exit(1)
		}
	case "dig":
		if err := dig.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "dig error: %v\n", err)
			os.Exit(1)
		}
	case "nc":
		if err := nc.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "nc error: %v\n", err)
			os.Exit(1)
		}
	case "curl":
		if err := curl.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "curl error: %v\n", err)
			os.Exit(1)
		}
	case "jq":
		if err := jq.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "jq error: %v\n", err)
			os.Exit(1)
		}
	case "mdns-publish":
		if err := mdns.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "mdns error: %v\n", err)
			os.Exit(1)
		}
	case "dhcp-request":
		if err := dhcp.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "dhcp error: %v\n", err)
			os.Exit(1)
		}
	case "sleep":
		if err := sleep.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "sleep error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		if err := http.Run(args); err != nil {
			Printer.Fprintf(os.Stderr, "http error: %v\n", err)
			os.Exit(1)
		}
	default:
		help()
		os.Exit(1)
	}
}

func help() {
	Printer.Println("Flywall Toolbox - Busybox style test utils")
	Printer.Println("Usage: invoke as 'agent', 'orca', 'prove', or 'toolbox <subcmd>'")
}
