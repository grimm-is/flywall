// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package configdoc

// Schema represents the complete documentation schema for HCL configuration.
type Schema struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Blocks      map[string]*Block `json:"blocks"`
	Attributes  []*Field          `json:"attributes,omitempty"`
}

// Block represents an HCL block type (e.g., interface, policy, zone).
type Block struct {
	Name          string   `json:"name"`
	HCLName       string   `json:"hcl_name"`
	Description   string   `json:"description"`
	Labels        []Label  `json:"labels,omitempty"`
	Fields        []*Field `json:"fields,omitempty"`
	Blocks        []*Block `json:"blocks,omitempty"`
	Deprecated    bool     `json:"deprecated,omitempty"`
	DeprecatedMsg string   `json:"deprecated_msg,omitempty"`
	Example       string   `json:"example,omitempty"`
	Multiple      bool     `json:"multiple,omitempty"` // Can appear multiple times
	GoType        string   `json:"go_type,omitempty"`
}

// Label represents a block label (e.g., interface "eth0" has one label).
type Label struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

// Field represents an HCL attribute field within a block.
type Field struct {
	Name          string   `json:"name"`
	HCLName       string   `json:"hcl_name"`
	GoName        string   `json:"go_name,omitempty"`
	Type          string   `json:"type"`
	HCLType       string   `json:"hcl_type"` // string, number, bool, list, map
	Description   string   `json:"description"`
	Required      bool     `json:"required"`
	Optional      bool     `json:"optional"`
	Default       string   `json:"default,omitempty"`
	DefaultValue  any      `json:"default_value,omitempty"`
	Deprecated    bool     `json:"deprecated,omitempty"`
	DeprecatedMsg string   `json:"deprecated_msg,omitempty"`
	Enum          []string `json:"enum,omitempty"`
	Example       string   `json:"example,omitempty"`
	Pattern       string   `json:"pattern,omitempty"` // Regex pattern for validation
	Min           *float64 `json:"min,omitempty"`
	Max           *float64 `json:"max,omitempty"`
	RefType       string   `json:"ref_type,omitempty"` // Reference to another block type
}

// ConfigSchema represents a schema document for the configuration.
type ConfigSchema struct {
	Schema      string                   `json:"$schema,omitempty"`
	ID          string                   `json:"$id,omitempty"`
	Title       string                   `json:"title,omitempty"`
	Description string                   `json:"description,omitempty"`
	Type        string                   `json:"type,omitempty" yaml:"type,omitempty"`
	Definitions map[string]*ConfigSchema `json:"$defs,omitempty" yaml:"$defs,omitempty"`
	Properties  map[string]*ConfigSchema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string                 `json:"required,omitempty" yaml:"required,omitempty"`

	// Field-level properties
	Items      *ConfigSchema `json:"items,omitempty" yaml:"items,omitempty"`
	Enum       []string      `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default    any           `json:"default,omitempty" yaml:"default,omitempty"`
	Pattern    string        `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Minimum    *float64      `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum    *float64      `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	Deprecated bool          `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Examples   []any         `json:"examples,omitempty" yaml:"examples,omitempty"`
	Ref        string        `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// FieldAnnotation represents parsed annotation comments.
// Annotations are special comments that provide additional metadata:
//
//	// @default: "builtin"
//	// @enum: builtin, external, monitor
//	// @example: "192.168.1.1/24"
//	// @deprecated: Use X instead
//	// @pattern: ^[a-z][a-z0-9_]*$
//	// @min: 1
//	// @max: 65535
type FieldAnnotation struct {
	Default       string
	DefaultValue  any
	Enum          []string
	Example       string
	Deprecated    bool
	DeprecatedMsg string
	Pattern       string
	Min           *float64
	Max           *float64
	RefType       string
}

// ParsedStruct represents a parsed Go struct with HCL tags.
type ParsedStruct struct {
	Name        string
	Doc         string
	Fields      []ParsedField
	IsBlock     bool   // Contains nested blocks
	PackagePath string // Source package
	SourceFile  string // Source file path
}

// ParsedField represents a parsed struct field.
type ParsedField struct {
	Name       string
	GoType     string
	HCLTag     HCLTag
	JSONTag    string
	Doc        string
	Annotation FieldAnnotation
}

// HCLTag represents parsed HCL struct tag.
type HCLTag struct {
	Name     string
	Optional bool
	Block    bool
	Label    bool
	Remain   bool
}
