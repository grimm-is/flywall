// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	// Create a shared node with an anchor
	sharedNode := &yaml.Node{
		Kind:   yaml.MappingNode,
		Anchor: "shared_thing", // Explicit anchor
		Tag:    "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "key"},
			{Kind: yaml.ScalarNode, Value: "value"},
		},
	}

	// Create root object
	root := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			// Key A (should define the anchor if A comes first)
			{Kind: yaml.ScalarNode, Value: "a_entry"},
			{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "nested"},
				sharedNode, // Usage 1
			}},

			// Key B
			{Kind: yaml.ScalarNode, Value: "b_entry"},
			sharedNode, // Usage 2
		},
	}

	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	err := enc.Encode(root)
	if err != nil {
		panic(err)
	}
}
