// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package configdoc

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// ToYAMLNode converts the ConfigSchema to a yaml.Node tree.
// It uses a reference counting approach to decide whether to put a schema in 'definitions' or inline it.
// - Usage > 1: Put in 'definitions' with anchor, use alias elsewhere.
// - Usage == 1: Inline (no anchor).
func ToYAMLNode(js *ConfigSchema) *yaml.Node {
	// 1. Count references
	counts := make(map[*ConfigSchema]int)

	// Visit all properties to count usages
	var counter func(*ConfigSchema)
	counter = func(s *ConfigSchema) {
		if s == nil {
			return
		}

		// If s is one of our definitions, increment count
		// (We only track ref counts for things that ARE definitions)
		isDef := false
		for _, def := range js.Definitions {
			if def == s {
				isDef = true
				break
			}
		}

		if isDef {
			counts[s]++
			// If we've already seen it, don't recurse (avoid infinite loops for recursive types)
			if counts[s] > 1 {
				return
			}
		} else {
			// Not a root definition, but might contain them?
			// Actually, in our structure, recursion only happens VIA definitions.
			// But nested inline objects need traversal.
		}

		// Recurse children
		if len(s.Properties) > 0 {
			for _, v := range s.Properties {
				counter(v)
			}
		}
		if s.Items != nil {
			counter(s.Items)
		}
	}

	for _, p := range js.Properties {
		counter(p)
	}

	// 2. Identify Shared Definitions
	sharedDefs := make(map[*ConfigSchema]bool) // Set of definitions that go into 'definitions' block

	// Also need to know their names
	defNamesMap := make(map[*ConfigSchema]string)
	for name, def := range js.Definitions {
		defNamesMap[def] = name
		if counts[def] > 1 {
			sharedDefs[def] = true
		}
	}

	// 3. Create Nodes for Shared Definitions
	sharedNodes := make(map[*ConfigSchema]*yaml.Node)
	for def, isShared := range sharedDefs {
		if isShared {
			name := defNamesMap[def]
			node := &yaml.Node{Kind: yaml.MappingNode}
			node.Anchor = name
			sharedNodes[def] = node
		}
	}

	// 4. Conversion Function
	var convert func(*ConfigSchema) *yaml.Node
	convert = func(s *ConfigSchema) *yaml.Node {
		if s == nil {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}
		}

		// Check if it's a shared definition
		if sharedDefs[s] {
			// It is shared.
			// The node in 'sharedNodes' is the Definition Site.
			// We should return an ALIAS to it.
			realNode := sharedNodes[s]
			return &yaml.Node{Kind: yaml.AliasNode, Value: realNode.Anchor, Alias: realNode}
		}

		// Not shared (or is shared but we are currently POPULATING it? No, populating happens separately)
		// Inline object.
		node := &yaml.Node{Kind: yaml.MappingNode}
		fillNode(node, s, convert)
		return node
	}

	// 5. Populate Shared Definitions Content
	// We sort names for determinism
	var sharedNames []string
	for def := range sharedDefs {
		sharedNames = append(sharedNames, defNamesMap[def])
	}
	sort.Strings(sharedNames)

	for _, name := range sharedNames {
		def := js.Definitions[name]
		node := sharedNodes[def]
		fillNode(node, def, convert)
	}

	// 6. Build Root Node
	rootNode := &yaml.Node{Kind: yaml.MappingNode}
	content := []*yaml.Node{}

	// Metadata
	if js.Schema != "" {
		content = append(content, scalar("$schema"), scalar(js.Schema))
	}
	if js.ID != "" {
		content = append(content, scalar("$id"), scalar(js.ID))
	}
	if js.Title != "" {
		content = append(content, scalar("title"), scalar(js.Title))
	}
	if js.Description != "" {
		content = append(content, scalar("description"), scalar(js.Description))
	}
	if js.Type != "" {
		content = append(content, scalar("type"), scalar(js.Type))
	}

	// Definitions Block (Only shared ones)
	if len(sharedNames) > 0 {
		defsNode := &yaml.Node{Kind: yaml.MappingNode}
		for _, name := range sharedNames {
			def := js.Definitions[name]
			defsNode.Content = append(defsNode.Content, scalar(name), sharedNodes[def])
		}
		content = append(content, scalar("definitions"), defsNode)
	}

	// Properties
	if len(js.Properties) > 0 {
		content = append(content, scalar("properties"), mapToNode(js.Properties, convert))
	}

	// Required (Root)
	if len(js.Required) > 0 {
		content = append(content, scalar("required"), strSliceToNode(js.Required))
	}

	rootNode.Content = content
	return rootNode
}

// fillNode populates a node's content from the schema.
func fillNode(node *yaml.Node, js *ConfigSchema, convert func(*ConfigSchema) *yaml.Node) {
	content := []*yaml.Node{}

	if js.Title != "" {
		content = append(content, scalar("title"), scalar(js.Title))
	}
	if js.Description != "" {
		content = append(content, scalar("description"), scalar(js.Description))
	}
	if js.Type != "" {
		content = append(content, scalar("type"), scalar(js.Type))
	}
	if js.Ref != "" {
		content = append(content, scalar("$ref"), scalar(js.Ref))
	}

	// Recursively handle maps
	if len(js.Properties) > 0 {
		content = append(content, scalar("properties"), mapToNode(js.Properties, convert))
	}

	// Definitions inside a schema? ignored.

	// Fields
	if js.Items != nil {
		content = append(content, scalar("items"), convert(js.Items))
	}

	if len(js.Enum) > 0 {
		content = append(content, scalar("enum"), strSliceToNode(js.Enum))
	}

	if js.Default != nil {
		content = append(content, scalar("default"), anyToNode(js.Default))
	}

	if js.Pattern != "" {
		content = append(content, scalar("pattern"), scalar(js.Pattern))
	}
	if js.Minimum != nil {
		content = append(content, scalar("minimum"), scalar(fmt.Sprintf("%g", *js.Minimum)))
	}
	if js.Maximum != nil {
		content = append(content, scalar("maximum"), scalar(fmt.Sprintf("%g", *js.Maximum)))
	}

	// Required fields for this object
	if len(js.Required) > 0 {
		content = append(content, scalar("required"), strSliceToNode(js.Required))
	}

	node.Content = content
}

// mapToNode converts properties map to a MappingNode
func mapToNode(m map[string]*ConfigSchema, convert func(*ConfigSchema) *yaml.Node) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m[k]
		valNode := convert(v)
		node.Content = append(node.Content, scalar(k), valNode)
	}
	return node
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
}

func strSliceToNode(s []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode}
	for _, v := range s {
		node.Content = append(node.Content, scalar(v))
	}
	return node
}

func anyToNode(v any) *yaml.Node {
	// Simple conversion for default values
	return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)}
}
