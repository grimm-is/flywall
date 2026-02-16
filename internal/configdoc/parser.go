// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package configdoc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Parser extracts documentation from Go source files containing HCL config structs.
type Parser struct {
	fset    *token.FileSet
	pkgs    map[string]*ast.Package
	structs map[string]*ParsedStruct
}

// NewParser creates a new documentation parser.
func NewParser() *Parser {
	return &Parser{
		fset:    token.NewFileSet(),
		pkgs:    make(map[string]*ast.Package),
		structs: make(map[string]*ParsedStruct),
	}
}

// ParseDir parses all Go files in a directory and extracts struct definitions.
func (p *Parser) ParseDir(dir string) error {
	pkgs, err := parser.ParseDir(p.fset, dir, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	for name, pkg := range pkgs {
		// Skip test packages
		if strings.HasSuffix(name, "_test") {
			continue
		}
		p.pkgs[name] = pkg
		p.extractStructs(pkg, dir)
	}

	return nil
}

// extractStructs extracts all struct definitions with HCL tags from a package.
func (p *Parser) extractStructs(pkg *ast.Package, dir string) {
	for filename, file := range pkg.Files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				// Check if this struct has any HCL tags
				if !p.hasHCLTags(structType) {
					continue
				}

				parsed := p.parseStruct(typeSpec.Name.Name, structType, genDecl.Doc)
				parsed.SourceFile = filepath.Base(filename)
				parsed.PackagePath = dir
				p.structs[typeSpec.Name.Name] = parsed
			}
		}
	}
}

// hasHCLTags checks if a struct has any fields with HCL tags.
func (p *Parser) hasHCLTags(s *ast.StructType) bool {
	if s.Fields == nil {
		return false
	}
	for _, field := range s.Fields.List {
		if field.Tag != nil {
			tag := field.Tag.Value
			if strings.Contains(tag, "hcl:") {
				return true
			}
		}
	}
	return false
}

// parseStruct parses a struct definition into a ParsedStruct.
func (p *Parser) parseStruct(name string, s *ast.StructType, docGroup *ast.CommentGroup) *ParsedStruct {
	parsed := &ParsedStruct{
		Name: name,
		Doc:  extractDocComment(docGroup),
	}

	if s.Fields == nil {
		return parsed
	}

	for _, field := range s.Fields.List {
		if len(field.Names) == 0 {
			continue // embedded field
		}

		parsedField := p.parseField(field)
		if parsedField.HCLTag.Name != "" {
			parsed.Fields = append(parsed.Fields, parsedField)
			if parsedField.HCLTag.Block {
				parsed.IsBlock = true
			}
		}
	}

	return parsed
}

// parseField parses a struct field into a ParsedField.
func (p *Parser) parseField(field *ast.Field) ParsedField {
	name := ""
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	}

	pf := ParsedField{
		Name:   name,
		GoType: typeToString(field.Type),
		Doc:    extractDocComment(field.Doc),
	}

	// Also include inline comments
	if field.Comment != nil {
		inlineDoc := extractDocComment(field.Comment)
		if pf.Doc == "" {
			pf.Doc = inlineDoc
		} else if inlineDoc != "" {
			pf.Doc = pf.Doc + " " + inlineDoc
		}
	}

	// Parse struct tags
	if field.Tag != nil {
		tag := field.Tag.Value
		tag = strings.Trim(tag, "`")

		pf.HCLTag = parseHCLTag(reflect.StructTag(tag).Get("hcl"))
		pf.JSONTag = reflect.StructTag(tag).Get("json")
	}

	// Parse annotations from doc comments
	pf.Annotation = parseAnnotations(pf.Doc)

	return pf
}

// parseHCLTag parses an HCL struct tag value.
func parseHCLTag(tag string) HCLTag {
	if tag == "" {
		return HCLTag{}
	}

	parts := strings.Split(tag, ",")
	ht := HCLTag{
		Name: parts[0],
	}

	for _, part := range parts[1:] {
		switch part {
		case "optional":
			ht.Optional = true
		case "block":
			ht.Block = true
		case "label":
			ht.Label = true
		case "remain":
			ht.Remain = true
		}
	}

	return ht
}

// parseAnnotations extracts structured annotations from doc comments.
func parseAnnotations(doc string) FieldAnnotation {
	ann := FieldAnnotation{}

	// Check for deprecated
	if strings.Contains(strings.ToLower(doc), "deprecated") {
		ann.Deprecated = true
		// Extract deprecation message
		re := regexp.MustCompile(`(?i)deprecated:?\s*(.*)`)
		if matches := re.FindStringSubmatch(doc); len(matches) > 1 {
			ann.DeprecatedMsg = strings.TrimSpace(matches[1])
		}
	}

	// Parse @annotations
	lines := strings.Split(doc, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// @default: value
		if strings.HasPrefix(line, "@default:") {
			ann.Default = strings.TrimSpace(strings.TrimPrefix(line, "@default:"))
		}

		// @enum: val1, val2, val3
		if strings.HasPrefix(line, "@enum:") {
			enumStr := strings.TrimSpace(strings.TrimPrefix(line, "@enum:"))
			for _, e := range strings.Split(enumStr, ",") {
				ann.Enum = append(ann.Enum, strings.TrimSpace(e))
			}
		}

		// @example: "192.168.1.1"
		if strings.HasPrefix(line, "@example:") {
			ann.Example = strings.TrimSpace(strings.TrimPrefix(line, "@example:"))
		}

		// @pattern: ^[a-z]+$
		if strings.HasPrefix(line, "@pattern:") {
			ann.Pattern = strings.TrimSpace(strings.TrimPrefix(line, "@pattern:"))
		}

		// @min: 1
		if strings.HasPrefix(line, "@min:") {
			if val, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "@min:")), 64); err == nil {
				ann.Min = &val
			}
		}

		// @max: 65535
		if strings.HasPrefix(line, "@max:") {
			if val, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "@max:")), 64); err == nil {
				ann.Max = &val
			}
		}

		// @ref: Zone
		if strings.HasPrefix(line, "@ref:") {
			ann.RefType = strings.TrimSpace(strings.TrimPrefix(line, "@ref:"))
		}
	}

	// Try to extract enum from inline comment pattern: // "a", "b", "c"
	enumRe := regexp.MustCompile(`"([^"]+)"(?:\s*,\s*"([^"]+)")+`)
	if matches := enumRe.FindAllStringSubmatch(doc, -1); len(matches) > 0 {
		// Only if no @enum annotation
		if len(ann.Enum) == 0 {
			for _, m := range matches {
				for i := 1; i < len(m); i++ {
					if m[i] != "" {
						ann.Enum = append(ann.Enum, m[i])
					}
				}
			}
		}
	}

	return ann
}

// extractDocComment extracts clean doc text from a comment group.
func extractDocComment(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}

	// Use go/doc to clean up the comment
	text := cg.Text()
	text = strings.TrimSpace(text)

	return text
}

// typeToString converts an AST type expression to a string.
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "unknown"
	}
}

// GetStruct returns a parsed struct by name.
func (p *Parser) GetStruct(name string) *ParsedStruct {
	return p.structs[name]
}

// GetAllStructs returns all parsed structs.
func (p *Parser) GetAllStructs() map[string]*ParsedStruct {
	return p.structs
}

// BuildSchema builds a complete documentation schema from parsed structs.
func (p *Parser) BuildSchema(rootType string) *Schema {
	schema := &Schema{
		Title:   "Flywall Configuration",
		Version: "1.0",
		Blocks:  make(map[string]*Block),
	}

	root := p.structs[rootType]
	if root == nil {
		return schema
	}

	schema.Description = root.Doc

	// Process root fields
	for _, field := range root.Fields {
		if field.HCLTag.Block {
			block := p.buildBlock(field)
			if block != nil {
				schema.Blocks[block.HCLName] = block
			}
		} else {
			schema.Attributes = append(schema.Attributes, p.buildField(field))
		}
	}

	return schema
}

// buildBlock builds a Block from a field that references another struct.
func (p *Parser) buildBlock(field ParsedField) *Block {
	// Get the referenced struct type
	typeName := field.GoType
	typeName = strings.TrimPrefix(typeName, "*")
	typeName = strings.TrimPrefix(typeName, "[]")

	refStruct := p.structs[typeName]

	block := &Block{
		Name:          field.Name,
		HCLName:       field.HCLTag.Name,
		Description:   cleanDescription(field.Doc),
		GoType:        field.GoType,
		Multiple:      strings.HasPrefix(field.GoType, "[]"),
		Deprecated:    field.Annotation.Deprecated,
		DeprecatedMsg: field.Annotation.DeprecatedMsg,
	}

	if refStruct != nil {
		if refStruct.Doc != "" && block.Description == "" {
			block.Description = refStruct.Doc
		}

		// Find label fields
		for _, f := range refStruct.Fields {
			if f.HCLTag.Label {
				block.Labels = append(block.Labels, Label{
					Name:        f.HCLTag.Name,
					Description: cleanDescription(f.Doc),
					Required:    true,
				})
			}
		}

		// Process nested fields and blocks
		for _, f := range refStruct.Fields {
			if f.HCLTag.Label {
				continue // Already handled
			}

			if f.HCLTag.Block {
				nestedBlock := p.buildBlock(f)
				if nestedBlock != nil {
					block.Blocks = append(block.Blocks, nestedBlock)
				}
			} else {
				block.Fields = append(block.Fields, p.buildField(f))
			}
		}
	}

	return block
}

// buildField builds a Field from a ParsedField.
func (p *Parser) buildField(pf ParsedField) *Field {
	f := &Field{
		Name:          pf.Name,
		HCLName:       pf.HCLTag.Name,
		GoName:        pf.Name,
		Type:          pf.GoType,
		HCLType:       goTypeToHCLType(pf.GoType),
		Description:   cleanDescription(pf.Doc),
		Required:      !pf.HCLTag.Optional,
		Optional:      pf.HCLTag.Optional,
		Default:       pf.Annotation.Default,
		Deprecated:    pf.Annotation.Deprecated,
		DeprecatedMsg: pf.Annotation.DeprecatedMsg,
		Enum:          pf.Annotation.Enum,
		Example:       pf.Annotation.Example,
		Pattern:       pf.Annotation.Pattern,
		Min:           pf.Annotation.Min,
		Max:           pf.Annotation.Max,
		RefType:       pf.Annotation.RefType,
	}

	return f
}

// goTypeToHCLType maps Go types to HCL types.
func goTypeToHCLType(goType string) string {
	goType = strings.TrimPrefix(goType, "*")

	if strings.HasPrefix(goType, "[]") {
		return "list(" + goTypeToHCLType(strings.TrimPrefix(goType, "[]")) + ")"
	}

	if strings.HasPrefix(goType, "map[") {
		return "map"
	}

	switch goType {
	case "string":
		return "string"
	case "bool":
		return "bool"
	case "int", "int8", "int16", "int32", "int64":
		return "number"
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "number"
	case "float32", "float64":
		return "number"
	default:
		return "object"
	}
}

// cleanDescription removes annotation lines from description.
func cleanDescription(doc string) string {
	lines := strings.Split(doc, "\n")
	var clean []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@") {
			continue
		}
		clean = append(clean, line)
	}
	result := strings.Join(clean, "\n")
	return strings.TrimSpace(result)
}
