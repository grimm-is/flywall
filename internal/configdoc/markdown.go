// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package configdoc

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMarkdown generates Markdown documentation from a Schema.
func GenerateMarkdown(schema *Schema) string {
	var sb strings.Builder

	// Title and description
	sb.WriteString(fmt.Sprintf("# %s\n\n", schema.Title))
	if schema.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", schema.Description))
	}
	sb.WriteString(fmt.Sprintf("**Schema Version:** %s\n\n", schema.Version))

	// Table of contents
	sb.WriteString("## Table of Contents\n\n")
	sb.WriteString("- [Global Attributes](#global-attributes)\n")

	// Sort blocks for consistent output
	blockNames := make([]string, 0, len(schema.Blocks))
	for name := range schema.Blocks {
		blockNames = append(blockNames, name)
	}
	sort.Strings(blockNames)

	for _, name := range blockNames {
		block := schema.Blocks[name]
		anchor := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		sb.WriteString(fmt.Sprintf("- [%s](#%s)\n", name, anchor))
		if block.Deprecated {
			sb.WriteString(" ⚠️ *deprecated*")
		}
	}
	sb.WriteString("\n")

	// Global attributes
	if len(schema.Attributes) > 0 {
		sb.WriteString("## Global Attributes\n\n")
		sb.WriteString("Top-level configuration attributes.\n\n")
		writeFieldsTable(&sb, schema.Attributes)
	}

	// Blocks
	for _, name := range blockNames {
		block := schema.Blocks[name]
		writeBlock(&sb, block, 2)
	}

	return sb.String()
}

// writeBlock writes a block's documentation.
func writeBlock(sb *strings.Builder, block *Block, level int) {
	heading := strings.Repeat("#", level)

	sb.WriteString(fmt.Sprintf("%s %s\n\n", heading, block.HCLName))

	if block.Deprecated {
		sb.WriteString(fmt.Sprintf("> ⚠️ **Deprecated:** %s\n\n", block.DeprecatedMsg))
	}

	if block.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", block.Description))
	}

	// Block syntax
	sb.WriteString("**Syntax:**\n\n```hcl\n")
	sb.WriteString(block.HCLName)
	for _, label := range block.Labels {
		sb.WriteString(fmt.Sprintf(" \"%s\"", label.Name))
	}
	sb.WriteString(" {\n")
	// Show a few example fields
	shown := 0
	for _, f := range block.Fields {
		if shown >= 3 {
			sb.WriteString("  # ...\n")
			break
		}
		sb.WriteString(fmt.Sprintf("  %s = %s\n", f.HCLName, exampleValue(f)))
		shown++
	}
	sb.WriteString("}\n```\n\n")

	// Labels
	if len(block.Labels) > 0 {
		sb.WriteString("**Labels:**\n\n")
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

	// Fields
	if len(block.Fields) > 0 {
		sb.WriteString("**Attributes:**\n\n")
		writeFieldsTable(sb, block.Fields)
	}

	// Nested blocks
	if len(block.Blocks) > 0 {
		sb.WriteString("**Nested Blocks:**\n\n")
		for _, nested := range block.Blocks {
			multiple := ""
			if nested.Multiple {
				multiple = " (multiple allowed)"
			}
			sb.WriteString(fmt.Sprintf("- `%s`%s - %s\n", nested.HCLName, multiple, truncateDesc(nested.Description, 80)))
		}
		sb.WriteString("\n")

		// Write nested block details
		for _, nested := range block.Blocks {
			writeBlock(sb, nested, level+1)
		}
	}
}

// writeFieldsTable writes a markdown table for fields.
func writeFieldsTable(sb *strings.Builder, fields []*Field) {
	sb.WriteString("| Attribute | Type | Required | Description |\n")
	sb.WriteString("|-----------|------|----------|-------------|\n")

	for _, f := range fields {
		req := "Yes"
		if f.Optional {
			req := "No"
			if f.Default != "" {
				req = fmt.Sprintf("No (default: `%s`)", f.Default)
			}
			_ = req
		}
		if f.Optional {
			if f.Default != "" {
				req = fmt.Sprintf("No (default: `%s`)", f.Default)
			} else {
				req = "No"
			}
		}

		desc := truncateDesc(f.Description, 100)
		if f.Deprecated {
			desc = "⚠️ *Deprecated.* " + desc
		}
		if len(f.Enum) > 0 {
			desc += fmt.Sprintf(" Values: `%s`", strings.Join(f.Enum, "`, `"))
		}

		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n",
			f.HCLName, f.HCLType, req, desc))
	}
	sb.WriteString("\n")
}

// exampleValue returns an example value for a field.
func exampleValue(f *Field) string {
	if f.Example != "" {
		return f.Example
	}
	if f.Default != "" {
		return f.Default
	}
	if len(f.Enum) > 0 {
		return fmt.Sprintf("\"%s\"", f.Enum[0])
	}

	switch f.HCLType {
	case "string":
		return "\"...\""
	case "bool":
		return "true"
	case "number":
		return "0"
	default:
		if strings.HasPrefix(f.HCLType, "list(") {
			return "[...]"
		}
		if f.HCLType == "map" {
			return "{...}"
		}
		return "..."
	}
}

// truncateDesc truncates a description to maxLen characters.
func truncateDesc(desc string, maxLen int) string {
	// Remove newlines
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.TrimSpace(desc)

	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen-3] + "..."
}

// GenerateQuickReference generates a compact YAML-style quick reference.
func GenerateQuickReference(schema *Schema) string {
	var sb strings.Builder

	sb.WriteString("# Flywall Configuration Quick Reference\n")
	sb.WriteString(fmt.Sprintf("# Schema Version: %s\n\n", schema.Version))

	// Global attributes
	if len(schema.Attributes) > 0 {
		sb.WriteString("# Global Attributes\n")
		for _, f := range schema.Attributes {
			writeQuickRefField(&sb, f, 0)
		}
		sb.WriteString("\n")
	}

	// Sort blocks
	blockNames := make([]string, 0, len(schema.Blocks))
	for name := range schema.Blocks {
		blockNames = append(blockNames, name)
	}
	sort.Strings(blockNames)

	for _, name := range blockNames {
		block := schema.Blocks[name]
		writeQuickRefBlock(&sb, block, 0)
	}

	return sb.String()
}

func writeQuickRefBlock(sb *strings.Builder, block *Block, indent int) {
	prefix := strings.Repeat("  ", indent)

	// Block header
	labelStr := ""
	for _, l := range block.Labels {
		labelStr += fmt.Sprintf(" \"<%s>\"", l.Name)
	}

	deprecated := ""
	if block.Deprecated {
		deprecated = " # DEPRECATED"
	}

	sb.WriteString(fmt.Sprintf("%s%s%s {%s\n", prefix, block.HCLName, labelStr, deprecated))

	// Fields
	for _, f := range block.Fields {
		writeQuickRefField(sb, f, indent+1)
	}

	// Nested blocks
	for _, nested := range block.Blocks {
		writeQuickRefBlock(sb, nested, indent+1)
	}

	sb.WriteString(fmt.Sprintf("%s}\n\n", prefix))
}

func writeQuickRefField(sb *strings.Builder, f *Field, indent int) {
	prefix := strings.Repeat("  ", indent)

	typeStr := f.HCLType
	if len(f.Enum) > 0 {
		typeStr = strings.Join(f.Enum, "|")
	}

	optStr := ""
	if f.Optional {
		optStr = "optional"
		if f.Default != "" {
			optStr = fmt.Sprintf("default=%s", f.Default)
		}
	} else {
		optStr = "required"
	}

	deprecated := ""
	if f.Deprecated {
		deprecated = " DEPRECATED"
	}

	sb.WriteString(fmt.Sprintf("%s%s = <%s>  # %s%s\n", prefix, f.HCLName, typeStr, optStr, deprecated))
}

// GenerateAIContext generates a comprehensive Markdown representation for AI context.
func GenerateAIContext(schema *Schema) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Configuration Schema\n\n", schema.Title))
	sb.WriteString("This document describes the configuration schema for Flywall. Use this to understand the structure, types, and allowed values for configuration files.\n\n")

	// Global attributes
	if len(schema.Attributes) > 0 {
		sb.WriteString("## Global Attributes\n\n")
		for _, f := range schema.Attributes {
			writeAIField(&sb, f, 0)
		}
		sb.WriteString("\n")
	}

	// Blocks
	sb.WriteString("## Configuration Blocks\n\n")
	blockNames := make([]string, 0, len(schema.Blocks))
	for name := range schema.Blocks {
		blockNames = append(blockNames, name)
	}
	sort.Strings(blockNames)

	for _, name := range blockNames {
		block := schema.Blocks[name]
		writeAIBlock(&sb, block, 0)
	}

	return sb.String()
}

func writeAIBlock(sb *strings.Builder, block *Block, level int) {
	prefix := strings.Repeat("  ", level)
	heading := strings.Repeat("#", level+3)

	sb.WriteString(fmt.Sprintf("%s %s `%s`\n\n", heading, prefix, block.HCLName))

	if block.Description != "" {
		sb.WriteString(fmt.Sprintf("%s%s\n\n", prefix, block.Description))
	}

	if len(block.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("%s**Labels:** ", prefix))
		for i, l := range block.Labels {
			if i > 0 {
				sb.WriteString(", ")
			}
			req := "optional"
			if l.Required {
				req = "required"
			}
			sb.WriteString(fmt.Sprintf("`%s` (%s)", l.Name, req))
		}
		sb.WriteString("\n\n")
	}

	if len(block.Fields) > 0 {
		sb.WriteString(fmt.Sprintf("%s**Attributes:**\n", prefix))
		for _, f := range block.Fields {
			writeAIField(sb, f, level)
		}
		sb.WriteString("\n")
	}

	if len(block.Blocks) > 0 {
		sb.WriteString(fmt.Sprintf("%s**Nested Blocks:**\n", prefix))
		for _, nested := range block.Blocks {
			writeAIBlock(sb, nested, level+1)
		}
	}
}

func writeAIField(sb *strings.Builder, f *Field, level int) {
	prefix := strings.Repeat("  ", level)

	req := "Optional"
	if !f.Optional {
		req = "Required"
	} else if f.Default != "" {
		req = fmt.Sprintf("Optional (default: `%s`)", f.Default)
	}

	desc := strings.ReplaceAll(f.Description, "\n", " ")

	sb.WriteString(fmt.Sprintf("%s- `%s` (%s): %s %s\n", prefix, f.HCLName, f.HCLType, req, desc))
	if len(f.Enum) > 0 {
		sb.WriteString(fmt.Sprintf("%s  - Allowed values: `%s`\n", prefix, strings.Join(f.Enum, "`, `")))
	}
}
