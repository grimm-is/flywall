// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// gen-config-docs generates documentation from HCL config struct definitions.
//
// Usage:
//
//	go run ./cmd/gen-config-docs -format=markdown -output=docs/config-reference.md
//	go run ./cmd/gen-config-docs -format=jsonschema -output=docs/config-schema.json
//	go run ./cmd/gen-config-docs -format=ai -output=docs/config-ai-context.md
//	go run ./cmd/gen-config-docs -format=quickref -output=docs/config-quickref.txt
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"grimm.is/flywall/internal/configdoc"
)

func main() {
	format := flag.String("format", "markdown", "Output format: markdown, jsonschema, yaml, ai, quickref, hugo, all")
	output := flag.String("output", "", "Output file (default: stdout, or docs/ for 'all', or docs-site/content/docs/configuration/reference/ for 'hugo')")
	configDir := flag.String("config-dir", "internal/config", "Directory containing config Go files")
	flag.Parse()

	// Parse config directory
	parser := configdoc.NewParser()
	if err := parser.ParseDir(*configDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config directory: %v\n", err)
		os.Exit(1)
	}

	// Build schema from Config root type
	schema := parser.BuildSchema("Config")

	switch *format {
	case "markdown":
		content := configdoc.GenerateMarkdown(schema)
		writeOutput(*output, content)

	case "jsonschema":
		js := configdoc.GenerateSchema(schema, false)
		content, err := configdoc.ConfigSchemaToJSON(js)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating JSON schema: %v\n", err)
			os.Exit(1)
		}
		writeOutput(*output, content)

	case "yaml":
		js := configdoc.GenerateSchema(schema, true)
		node := configdoc.ToYAMLNode(js)
		data, err := yaml.Marshal(node)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating YAML schema: %v\n", err)
			os.Exit(1)
		}
		writeOutput(*output, string(data))

	case "ai":
		content := configdoc.GenerateAIContext(schema)
		writeOutput(*output, content)

	case "quickref":
		content := configdoc.GenerateQuickReference(schema)
		writeOutput(*output, content)

	case "hugo":
		outputDir := *output
		if outputDir == "" {
			outputDir = "docs-site/content/docs/configuration/reference"
		}
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}

		hugoOutput := configdoc.GenerateHugo(schema)
		for name, content := range hugoOutput.Files {
			path := filepath.Join(outputDir, name)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
				os.Exit(1)
			}
			fmt.Printf("Generated %s\n", path)
		}

	case "all":
		outputDir := *output
		if outputDir == "" {
			outputDir = "docs"
		}
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}

		// Generate all formats
		files := map[string]string{
			"config-reference.md":  configdoc.GenerateMarkdown(schema),
			"config-quickref.txt":  configdoc.GenerateQuickReference(schema),
			"config-ai-context.md": configdoc.GenerateAIContext(schema),
		}

		// JSON Schema
		js := configdoc.GenerateSchema(schema, false)
		jsContent, err := configdoc.ConfigSchemaToJSON(js)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating JSON schema: %v\n", err)
			os.Exit(1)
		}
		files["config-schema.json"] = jsContent

		for name, content := range files {
			path := filepath.Join(outputDir, name)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
				os.Exit(1)
			}
			fmt.Printf("Generated %s\n", path)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", *format)
		os.Exit(1)
	}
}

func writeOutput(path, content string) {
	if path == "" {
		fmt.Print(content)
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			os.Exit(1)
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s\n", path)
}
