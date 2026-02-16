// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package configdoc

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// HugoOutput represents a set of Hugo-compatible markdown files.
type HugoOutput struct {
	Files map[string]string // path -> content
}

// GenerateHugo generates Hugo-compatible markdown files with front matter.
// It creates:
// - _index.md (overview)
// - One file per top-level block (interface.md, zone.md, etc.)
func GenerateHugo(schema *Schema) *HugoOutput {
	output := &HugoOutput{
		Files: make(map[string]string),
	}

	// Generate index page
	output.Files["_index.md"] = generateHugoIndex(schema)

	// Generate global attributes page
	if len(schema.Attributes) > 0 {
		output.Files["global.md"] = generateHugoGlobal(schema)
	}

	// Generate a page per top-level block
	blockNames := make([]string, 0, len(schema.Blocks))
	for name := range schema.Blocks {
		blockNames = append(blockNames, name)
	}
	sort.Strings(blockNames)

	for i, name := range blockNames {
		block := schema.Blocks[name]
		filename := strings.ToLower(name) + ".md"
		output.Files[filename] = generateHugoBlock(block, i+20) // weight starts at 20
	}

	return output
}

// generateHugoIndex creates the _index.md file with overview
func generateHugoIndex(schema *Schema) string {
	var sb strings.Builder

	// Front matter
	sb.WriteString("---\n")
	sb.WriteString("title: \"Configuration Reference\"\n")
	sb.WriteString("linkTitle: \"Reference\"\n")
	sb.WriteString("weight: 10\n")
	sb.WriteString("description: >\n")
	sb.WriteString("  Complete reference of all HCL configuration options.\n")
	sb.WriteString("---\n\n")

	// Introduction
	sb.WriteString("This reference is auto-generated from the Flywall source code.\n\n")
	sb.WriteString(fmt.Sprintf("**Schema Version:** %s\n\n", schema.Version))

	// Quick navigation
	sb.WriteString("## Configuration Blocks\n\n")
	sb.WriteString("| Block | Description |\n")
	sb.WriteString("|-------|-------------|\n")

	blockNames := make([]string, 0, len(schema.Blocks))
	for name := range schema.Blocks {
		blockNames = append(blockNames, name)
	}
	sort.Strings(blockNames)

	for _, name := range blockNames {
		block := schema.Blocks[name]
		desc := truncateDesc(block.Description, 60)
		link := fmt.Sprintf("[%s]({{< relref \"%s\" >}})", name, strings.ToLower(name))
		deprecated := ""
		if block.Deprecated {
			deprecated = " ⚠️"
		}
		sb.WriteString(fmt.Sprintf("| %s%s | %s |\n", link, deprecated, desc))
	}
	sb.WriteString("\n")

	// Global attributes link
	if len(schema.Attributes) > 0 {
		sb.WriteString("## Global Attributes\n\n")
		sb.WriteString("Top-level configuration attributes are documented in [Global Settings]({{< relref \"global\" >}}).\n\n")
	}

	// Example basic config
	sb.WriteString("## Minimal Example\n\n")
	sb.WriteString("```hcl\n")
	sb.WriteString("schema_version = \"1.0\"\n")
	sb.WriteString("ip_forwarding = true\n\n")
	sb.WriteString("interface \"eth0\" {\n")
	sb.WriteString("  zone = \"WAN\"\n")
	sb.WriteString("  dhcp = true\n")
	sb.WriteString("}\n\n")
	sb.WriteString("interface \"eth1\" {\n")
	sb.WriteString("  zone = \"LAN\"\n")
	sb.WriteString("  ipv4 = [\"192.168.1.1/24\"]\n")
	sb.WriteString("}\n\n")
	sb.WriteString("zone \"WAN\" {}\n")
	sb.WriteString("zone \"LAN\" {\n")
	sb.WriteString("  management { web_ui = true }\n")
	sb.WriteString("}\n\n")
	sb.WriteString("policy \"LAN\" \"WAN\" {\n")
	sb.WriteString("  rule \"allow\" { action = \"accept\" }\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	return sb.String()
}

// generateHugoGlobal creates the global.md file for global attributes
func generateHugoGlobal(schema *Schema) string {
	var sb strings.Builder

	// Front matter
	sb.WriteString("---\n")
	sb.WriteString("title: \"Global Settings\"\n")
	sb.WriteString("linkTitle: \"Global\"\n")
	sb.WriteString("weight: 5\n")
	sb.WriteString("description: >\n")
	sb.WriteString("  Top-level configuration attributes.\n")
	sb.WriteString("---\n\n")

	sb.WriteString("These attributes are set at the top level of your configuration file.\n\n")

	sb.WriteString("## Attributes\n\n")
	writeHugoFieldsTable(&sb, schema.Attributes)

	// Example
	sb.WriteString("## Example\n\n")
	sb.WriteString("```hcl\n")
	sb.WriteString("schema_version = \"1.0\"\n")
	sb.WriteString("ip_forwarding  = true\n")
	sb.WriteString("ipv6_forwarding = false\n")
	sb.WriteString("state_dir      = \"/var/lib/flywall\"\n")
	sb.WriteString("log_dir        = \"/var/log/flywall\"\n")
	sb.WriteString("```\n")

	return sb.String()
}

// generateHugoBlock creates a page for a single top-level block
func generateHugoBlock(block *Block, weight int) string {
	var sb strings.Builder

	// Front matter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: \"%s\"\n", block.HCLName))
	sb.WriteString(fmt.Sprintf("linkTitle: \"%s\"\n", block.HCLName))
	sb.WriteString(fmt.Sprintf("weight: %d\n", weight))
	if block.Description != "" {
		// Escape description for YAML
		desc := strings.ReplaceAll(block.Description, "\"", "\\\"")
		desc = strings.ReplaceAll(desc, "\n", " ")
		if len(desc) > 100 {
			desc = desc[:97] + "..."
		}
		sb.WriteString("description: >\n")
		sb.WriteString(fmt.Sprintf("  %s\n", desc))
	}
	sb.WriteString("---\n\n")

	// Deprecation warning
	if block.Deprecated {
		sb.WriteString("{{% alert title=\"Deprecated\" color=\"warning\" %}}\n")
		sb.WriteString(block.DeprecatedMsg)
		sb.WriteString("\n{{% /alert %}}\n\n")
	}

	// Description
	if block.Description != "" {
		sb.WriteString(block.Description)
		sb.WriteString("\n\n")
	}

	// Syntax
	sb.WriteString("## Syntax\n\n")
	sb.WriteString("```hcl\n")
	sb.WriteString(block.HCLName)
	for _, label := range block.Labels {
		sb.WriteString(fmt.Sprintf(" \"%s\"", label.Name))
	}
	sb.WriteString(" {\n")
	shown := 0
	for _, f := range block.Fields {
		if shown >= 5 {
			sb.WriteString("  # ...\n")
			break
		}
		sb.WriteString(fmt.Sprintf("  %s = %s\n", f.HCLName, exampleValue(f)))
		shown++
	}
	// Show nested block hints
	for _, nested := range block.Blocks {
		if shown >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("\n  %s { ... }\n", nested.HCLName))
		shown++
	}
	sb.WriteString("}\n```\n\n")

	// Labels
	if len(block.Labels) > 0 {
		sb.WriteString("## Labels\n\n")
		sb.WriteString("| Label | Description | Required |\n")
		sb.WriteString("|-------|-------------|----------|\n")
		for _, label := range block.Labels {
			req := "Yes"
			if !label.Required {
				req = "No"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", label.Name, label.Description, req))
		}
		sb.WriteString("\n")
	}

	// Attributes
	if len(block.Fields) > 0 {
		sb.WriteString("## Attributes\n\n")
		writeHugoFieldsTable(&sb, block.Fields)
	}

	// Nested blocks
	if len(block.Blocks) > 0 {
		sb.WriteString("## Nested Blocks\n\n")
		for _, nested := range block.Blocks {
			writeHugoNestedBlock(&sb, nested, 3)
		}
	}

	return sb.String()
}

// writeHugoNestedBlock writes a nested block's documentation
func writeHugoNestedBlock(sb *strings.Builder, block *Block, level int) {
	heading := strings.Repeat("#", level)

	sb.WriteString(fmt.Sprintf("%s %s\n\n", heading, block.HCLName))

	if block.Deprecated {
		sb.WriteString(fmt.Sprintf("> ⚠️ **Deprecated:** %s\n\n", block.DeprecatedMsg))
	}

	if block.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", block.Description))
	}

	// Syntax
	sb.WriteString("```hcl\n")
	sb.WriteString(block.HCLName)
	for _, label := range block.Labels {
		sb.WriteString(fmt.Sprintf(" \"%s\"", label.Name))
	}
	sb.WriteString(" {\n")
	for i, f := range block.Fields {
		if i >= 3 {
			sb.WriteString("  # ...\n")
			break
		}
		sb.WriteString(fmt.Sprintf("  %s = %s\n", f.HCLName, exampleValue(f)))
	}
	sb.WriteString("}\n```\n\n")

	// Labels
	if len(block.Labels) > 0 {
		sb.WriteString("**Labels:**\n\n")
		for _, label := range block.Labels {
			req := "required"
			if !label.Required {
				req = "optional"
			}
			sb.WriteString(fmt.Sprintf("- `%s` (%s) - %s\n", label.Name, req, label.Description))
		}
		sb.WriteString("\n")
	}

	// Attributes
	if len(block.Fields) > 0 {
		sb.WriteString("**Attributes:**\n\n")
		writeHugoFieldsTable(sb, block.Fields)
	}

	// Recursively write further nested blocks
	for _, nested := range block.Blocks {
		writeHugoNestedBlock(sb, nested, level+1)
	}
}

// writeHugoFieldsTable writes a markdown table for fields with Hugo formatting
func writeHugoFieldsTable(sb *strings.Builder, fields []*Field) {
	sb.WriteString("| Attribute | Type | Required | Description |\n")
	sb.WriteString("|-----------|------|----------|-------------|\n")

	for _, f := range fields {
		req := "Yes"
		if f.Optional {
			if f.Default != "" {
				req = fmt.Sprintf("No (default: `%s`)", f.Default)
			} else {
				req = "No"
			}
		}

		desc := truncateDesc(f.Description, 80)
		if f.Deprecated {
			desc = "⚠️ *Deprecated.* " + desc
		}
		if len(f.Enum) > 0 {
			desc += fmt.Sprintf(" Values: `%s`", strings.Join(f.Enum, "`, `"))
		}

		// Escape pipe characters in description
		desc = strings.ReplaceAll(desc, "|", "\\|")

		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n",
			f.HCLName, f.HCLType, req, desc))
	}
	sb.WriteString("\n")
}

// WriteHugoFiles writes the Hugo output to a directory
func (h *HugoOutput) WriteToDir(dir string) error {
	for path, content := range h.Files {
		fullPath := filepath.Join(dir, path)
		if err := writeFileWithDir(fullPath, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func writeFileWithDir(path, content string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := makeDir(dir); err != nil {
			return err
		}
	}
	return writeFile(path, content)
}

// These will be replaced by os functions in main.go
var makeDir = func(path string) error { return nil }
var writeFile = func(path, content string) error { return nil }
