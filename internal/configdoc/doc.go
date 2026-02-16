// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

// Package configdoc provides tools for generating documentation from HCL config struct definitions.
//
// It parses Go source files containing struct definitions with HCL tags and generates
// documentation in multiple formats:
//   - Markdown for human consumption
//   - JSON Schema for AI/tooling consumption
//   - YAML reference for quick lookup
//
// The package extracts information from:
//   - Go doc comments on types and fields
//   - HCL struct tags (field names, optional/block attributes)
//   - JSON struct tags (for API compatibility reference)
//   - Special annotation comments (deprecated, default values, examples)
package configdoc
