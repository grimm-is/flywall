// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// gen-migrations generates migration stubs by diffing versioned JSON schemas.
//
// Usage:
//
//	go run ./cmd/gen-migrations 1.0 1.1
//	# Diffs schema/v1.0.json vs schema/v1.1.json and outputs migration stub
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// JSONSchema represents a simplified JSON/YAML Schema structure for diffing.
type JSONSchema struct {
	Properties map[string]Property `json:"properties" yaml:"properties"`
	Defs       map[string]Property `json:"$defs" yaml:"definitions"`
}

// Property represents a schema property.
type Property struct {
	Description string              `json:"description" yaml:"description"`
	Type        string              `json:"type" yaml:"type"`
	Ref         string              `json:"$ref" yaml:"$ref"`
	Properties  map[string]Property `json:"properties" yaml:"properties"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: gen-migrations <from_version> <to_version>\n")
		fmt.Fprintf(os.Stderr, "Example: gen-migrations 1.0 1.1\n")
		os.Exit(1)
	}

	fromVersion := os.Args[1]
	toVersion := os.Args[2]

	fromPath := filepath.Join("schema", fmt.Sprintf("v%s.yaml", fromVersion))
	if _, err := os.Stat(fromPath); os.IsNotExist(err) {
		fromPath = filepath.Join("schema", fmt.Sprintf("v%s.json", fromVersion))
	}

	toPath := filepath.Join("schema", fmt.Sprintf("v%s.yaml", toVersion))
	if _, err := os.Stat(toPath); os.IsNotExist(err) {
		toPath = filepath.Join("schema", fmt.Sprintf("v%s.json", toVersion))
	}

	fromSchema, err := loadSchema(fromPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", fromPath, err)
		os.Exit(1)
	}

	toSchema, err := loadSchema(toPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", toPath, err)
		os.Exit(1)
	}

	// Generate diff
	ops := diffSchemas(fromSchema, toSchema)

	// Output HCL stub
	fmt.Printf("# Auto-generated migration stub from %s to %s\n", fromVersion, toVersion)
	fmt.Printf("# Review and adjust before committing\n\n")
	fmt.Printf("migration \"%s\" \"%s\" {\n", fromVersion, toVersion)
	fmt.Printf("  description = \"Migration from %s to %s\"\n\n", fromVersion, toVersion)

	for _, op := range ops {
		fmt.Printf("  operation \"%s\" \"%s\" {\n", op.Type, op.Target)
		if op.Description != "" {
			fmt.Printf("    # %s\n", op.Description)
		}
		fmt.Printf("  }\n")
	}

	fmt.Printf("}\n")
}

func loadSchema(path string) (*JSONSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var schema JSONSchema
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		if err := yaml.Unmarshal(data, &schema); err != nil {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, err
		}
	}

	return &schema, nil
}

// MigrationOp represents a detected schema change.
type MigrationOp struct {
	Type        string
	Target      string
	Description string
}

func diffSchemas(from, to *JSONSchema) []MigrationOp {
	var ops []MigrationOp

	fromProps := getTopLevelBlocks(from)
	toProps := getTopLevelBlocks(to)

	// Find added blocks
	for name := range toProps {
		if _, exists := fromProps[name]; !exists {
			ops = append(ops, MigrationOp{
				Type:        "add_block",
				Target:      name,
				Description: toProps[name],
			})
		}
	}

	// Find removed blocks
	for name := range fromProps {
		if _, exists := toProps[name]; !exists {
			ops = append(ops, MigrationOp{
				Type:        "remove_block",
				Target:      name,
				Description: fromProps[name],
			})
		}
	}

	// Sort for deterministic output
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Type != ops[j].Type {
			return ops[i].Type < ops[j].Type
		}
		return ops[i].Target < ops[j].Target
	})

	return ops
}

// getTopLevelBlocks extracts block-type properties (those with $ref).
func getTopLevelBlocks(schema *JSONSchema) map[string]string {
	blocks := make(map[string]string)
	for name, prop := range schema.Properties {
		isBlock := prop.Ref != "" || prop.Type == "array" || prop.Type == "object"
		if isBlock {
			desc := prop.Description
			if desc == "" {
				desc = name + " block"
			}
			blocks[name] = strings.Split(desc, "\n")[0] // First line only
		}
	}
	return blocks
}
