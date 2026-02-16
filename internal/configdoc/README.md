# configdoc - HCL Configuration Documentation Generator

This package generates documentation from Go struct definitions that use HCL tags.

## Usage

```bash
# Generate all formats
go run ./cmd/gen-config-docs -format=all -output=docs/config

# Generate specific format
go run ./cmd/gen-config-docs -format=markdown -output=docs/config-reference.md
go run ./cmd/gen-config-docs -format=jsonschema -output=docs/config-schema.json
go run ./cmd/gen-config-docs -format=ai -output=docs/config-ai-context.md
go run ./cmd/gen-config-docs -format=quickref -output=docs/config-quickref.txt
```

## Output Formats

| Format | File | Purpose |
|--------|------|---------|
| `markdown` | config-reference.md | Human-readable reference documentation |
| `jsonschema` | config-schema.json | JSON Schema for validation and IDE support |
| `ai` | config-ai-context.md | Compact format optimized for LLM context |
| `quickref` | config-quickref.txt | Quick reference with all fields |

## Annotation System

The parser extracts documentation from Go doc comments. You can add special annotations
to provide additional metadata:

### Supported Annotations

```go
// Field description goes here.
// Multi-line descriptions are supported.
//
// @default: "builtin"
// @enum: builtin, external, monitor
// @example: "192.168.1.1/24"
// @pattern: ^[a-z][a-z0-9_]*$
// @min: 1
// @max: 65535
// @ref: Zone
FieldName string `hcl:"field_name,optional"`
```

| Annotation | Description |
|------------|-------------|
| `@default:` | Default value when not specified |
| `@enum:` | Comma-separated list of valid values |
| `@example:` | Example value for documentation |
| `@pattern:` | Regex pattern for validation |
| `@min:` | Minimum numeric value |
| `@max:` | Maximum numeric value |
| `@ref:` | Reference to another type (for nested objects) |

### Deprecation

Fields are automatically marked as deprecated if their doc comment contains "Deprecated":

```go
// Deprecated: Use NewField instead
OldField string `hcl:"old_field,optional"`
```

### Inline Comments

Both doc comments (above the field) and inline comments (on the same line) are captured:

```go
// This is a doc comment
FieldName string `hcl:"field_name"` // This inline comment is also captured
```

## Adding Documentation to Config Structs

1. Add descriptive doc comments to struct types and fields
2. Use annotations for default values, enums, and examples
3. Mark deprecated fields with "Deprecated:" prefix
4. Run `go run ./cmd/gen-config-docs -format=all` to regenerate docs

## Example

```go
// Interface represents a physical network interface configuration.
type Interface struct {
    // Name is the interface identifier (e.g., "eth0", "wlan0").
    // @example: "eth0"
    Name string `hcl:"name,label"`

    // Zone assigns this interface to a security zone.
    // @ref: Zone
    Zone string `hcl:"zone,optional"`

    // DHCP enables DHCP client on this interface.
    // @default: false
    DHCP bool `hcl:"dhcp,optional"`

    // DHCPClient specifies how DHCP client is managed.
    // @enum: builtin, external, monitor
    // @default: "builtin"
    DHCPClient string `hcl:"dhcp_client,optional"`

    // MTU sets the Maximum Transmission Unit.
    // @min: 576
    // @max: 9000
    // @default: 1500
    MTU int `hcl:"mtu,optional"`
}
```
