// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package main

import (
	"os"

	"grimm.is/flywall/internal/i18n"
	"grimm.is/flywall/internal/tui"
)

var Printer = i18n.NewCLIPrinter()

// Test Config Struct
type FirewallRule struct {
	Name      string `tui:"title=Rule Name,desc=Unique identifier,validate=required"`
	Protocol  string `tui:"title=Protocol,options=TCP:tcp,UDP:udp,ICMP:icmp"`
	Source    string `tui:"title=Source CIDR,validate=cidr"`
	Logging   bool   `tui:"title=Enable Logging,desc=Log to nflog group 100"`
	Reflected bool   `tui:"title=NAT Reflection,desc=Enable Hairpin NAT"`
}

func main() {
	// 1. Create default data
	rule := &FirewallRule{
		Protocol: "tcp",
		Logging:  true,
		Source:   "10.0.0.1/32",
	}

	// 2. Generate Form
	form := tui.AutoForm(rule)

	// 3. Run interacting
	Printer.Println("Launch AutoForm verification...")
	err := form.Run()
	if err != nil {
		Printer.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// 4. Print Result
	Printer.Printf("\n--- Result ---\n")
	Printer.Printf("Name: %s\n", rule.Name)
	Printer.Printf("Proto: %s\n", rule.Protocol)
	Printer.Printf("Source: %s\n", rule.Source)
	Printer.Printf("Logging: %v\n", rule.Logging)
	Printer.Printf("Reflected: %v\n", rule.Reflected)
}
