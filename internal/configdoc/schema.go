// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package configdoc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GenerateSchema generates a generic Schema from the documentation Schema.
func GenerateSchema(schema *Schema, useAnchors bool) *ConfigSchema {
	js := &ConfigSchema{
		Schema:      "https://json-schema.org/draft/2020-12/schema",
		ID:          "https://flywall.dev/schemas/config.json", // Keep ID for compatibility
		Title:       schema.Title,
		Description: schema.Description,
		Type:        "object",
		Properties:  make(map[string]*ConfigSchema),
		Definitions: make(map[string]*ConfigSchema),
	}

	// 1. First pass: Pre-allocate all definitions to establish stable pointers
	for name := range schema.Blocks {
		defName := sanitizeDefName(name)
		js.Definitions[defName] = &ConfigSchema{}
	}

	// 2. Second pass: Populate definitions
	for name, block := range schema.Blocks {
		defName := sanitizeDefName(name)
		defSchema := js.Definitions[defName]
		populateBlockSchema(defSchema, block, js.Definitions, useAnchors)
	}

	// 3. Add properties (global attributes)
	for _, attr := range schema.Attributes {
		js.Properties[attr.HCLName] = fieldToSchema(attr, js.Definitions, useAnchors)
		if attr.Required {
			js.Required = append(js.Required, attr.HCLName)
		}
	}

	// 4. Add blocks as properties
	for name, block := range schema.Blocks {
		defName := sanitizeDefName(name)
		defSchema := js.Definitions[defName]

		if useAnchors {
			// YAML Anchor: Use the pointer directly
			if block.Multiple {
				js.Properties[block.HCLName] = &ConfigSchema{
					Type:        "array",
					Description: block.Description,
					Items:       defSchema, // Pointer to def -> *Anchor
				}
			} else {
				// Single block: property IS the def (alias)
				js.Properties[block.HCLName] = defSchema
			}
		} else {
			// JSON Schema Ref: Use string ref wrapper
			refStr := "#/$defs/" + defName
			if block.Multiple {
				js.Properties[block.HCLName] = &ConfigSchema{
					Type:        "array",
					Description: block.Description,
					Items:       &ConfigSchema{Ref: refStr},
				}
			} else {
				js.Properties[block.HCLName] = &ConfigSchema{
					Ref:         refStr,
					Description: block.Description,
				}
			}
		}
	}

	return js
}

// populateBlockSchema populates an existing ConfigSchema struct from a Block.
func populateBlockSchema(js *ConfigSchema, block *Block, defs map[string]*ConfigSchema, useAnchors bool) {
	js.Title = block.Name
	js.Description = block.Description
	js.Type = "object"
	js.Properties = make(map[string]*ConfigSchema)
	js.Deprecated = block.Deprecated

	var required []string

	// Add labels as properties (they're typically required)
	for _, label := range block.Labels {
		js.Properties["_label_"+label.Name] = &ConfigSchema{
			Type:        "string",
			Description: fmt.Sprintf("Block label: %s", label.Description),
		}
		if label.Required {
			required = append(required, "_label_"+label.Name)
		}
	}

	// Add fields
	for _, field := range block.Fields {
		js.Properties[field.HCLName] = fieldToSchema(field, defs, useAnchors)
		if field.Required {
			required = append(required, field.HCLName)
		}
	}

	// Add nested blocks
	for _, nested := range block.Blocks {
		// Recursive blocks: if we used anchors for them, they'd need to be in defs.
		// For now, simple recursion.
		nestedSchema := &ConfigSchema{}
		populateBlockSchema(nestedSchema, nested, defs, useAnchors)

		if nested.Multiple {
			js.Properties[nested.HCLName] = &ConfigSchema{
				Type:        "array",
				Description: nested.Description,
				Items:       nestedSchema,
			}
		} else {
			js.Properties[nested.HCLName] = nestedSchema
		}
	}

	if len(required) > 0 {
		js.Required = required
	}
}

// fieldToSchema converts a Field to a generic Schema property.
func fieldToSchema(field *Field, defs map[string]*ConfigSchema, useAnchors bool) *ConfigSchema {
	js := &ConfigSchema{
		Description: field.Description,
		Deprecated:  field.Deprecated,
	}

	// Set type based on HCL type
	switch {
	case field.HCLType == "string":
		js.Type = "string"
		if len(field.Enum) > 0 {
			js.Enum = field.Enum
		}
		if field.Pattern != "" {
			js.Pattern = field.Pattern
		}

	case field.HCLType == "bool":
		js.Type = "boolean"

	case field.HCLType == "number":
		js.Type = "number"
		js.Minimum = field.Min
		js.Maximum = field.Max

	case strings.HasPrefix(field.HCLType, "list("):
		js.Type = "array"
		// Extract inner type
		inner := strings.TrimPrefix(field.HCLType, "list(")
		inner = strings.TrimSuffix(inner, ")")

		// Handling list of objects ref?
		if inner == "object" && field.RefType != "" {
			// Special handling for list(object) which is a Ref
			defName := sanitizeDefName(field.RefType)
			if useAnchors && defs != nil {
				if def, ok := defs[defName]; ok {
					js.Items = def
				} else {
					// Fallback if def not found yet
					js.Items = &ConfigSchema{Type: "object"}
				}
			} else {
				js.Items = &ConfigSchema{Ref: "#/$defs/" + defName}
			}
		} else {
			js.Items = &ConfigSchema{Type: hclTypeToJSONType(inner)}
		}

	case field.HCLType == "map":
		js.Type = "object"

	case field.HCLType == "object":
		js.Type = "object"
		if field.RefType != "" {
			defName := sanitizeDefName(field.RefType)
			if useAnchors && defs != nil {
				if def, ok := defs[defName]; ok {
					// Return the definition directly for anchor
					return def
				}
			}
			js.Ref = "#/$defs/" + defName
		}

	default:
		js.Type = "string"
	}

	// Add default
	if field.Default != "" {
		js.Default = parseDefaultValue(field.Default, js.Type)
	}

	// Add example
	if field.Example != "" {
		js.Examples = []any{field.Example}
	}

	return js
}

// hclTypeToJSONType converts HCL type to JSON Schema type.
func hclTypeToJSONType(hclType string) string {
	switch hclType {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "number":
		return "number"
	default:
		return "string"
	}
}

// sanitizeDefName sanitizes a definition name for JSON Schema.
func sanitizeDefName(name string) string {
	return name
}

// parseDefaultValue parses a default value string to the appropriate type.
func parseDefaultValue(def string, jsonType string) any {
	def = strings.Trim(def, "\"")

	switch jsonType {
	case "boolean":
		return def == "true"
	case "number":
		var n float64
		fmt.Sscanf(def, "%f", &n)
		return n
	default:
		return def
	}
}

// ConfigSchemaToJSON converts a ConfigSchema to pretty-printed JSON.
func ConfigSchemaToJSON(js *ConfigSchema) (string, error) {
	data, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
