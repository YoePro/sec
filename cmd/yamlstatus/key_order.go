package main

import (
	"sort"

	"gopkg.in/yaml.v3"
)

// integrationKeyOrder defines the shared fmt/edit layout (IDEAS.md §4.1).
// Unknown extension fields follow these keys in their original relative order.
var integrationKeyOrder = []string{
	"id", "area", "feature", "status",
	"integrated", "audited", "reviewed", "revised", "updated", "created",
	"document_revision", "repository_baseline", "summary",
	"rules", "code", "tests", "implemented", "partial", "remaining",
	"unimplemented", "required_tests", "verification",
}

// orderedIntegrationRoot copies only the document containers that hold actual
// integrations. Fragment headers, registry metadata and nested mappings are not
// integration fields. The source nodes (including comments/styles) stay intact.
func orderedIntegrationRoot(root *yaml.Node) *yaml.Node {
	copyNode := func(node *yaml.Node) *yaml.Node {
		copy := *node
		copy.Content = append([]*yaml.Node(nil), node.Content...)
		return &copy
	}
	orderList := func(node *yaml.Node) *yaml.Node {
		copy := copyNode(node)
		for i, item := range copy.Content {
			copy.Content[i] = orderedIntegration(item)
		}
		return copy
	}
	if root.Kind == yaml.SequenceNode {
		return orderList(root)
	}
	if root.Kind == yaml.MappingNode {
		if integrations := mappingValue(root, "integrations"); integrations != nil && integrations.Kind == yaml.SequenceNode {
			copy := copyNode(root)
			for i := 1; i < len(copy.Content); i += 2 {
				if copy.Content[i] == integrations {
					copy.Content[i] = orderList(integrations)
				}
			}
			return copy
		}
		if mappingValue(root, "id") != nil {
			return orderedIntegration(root)
		}
	}
	return root
}

// orderedIntegration stably orders key/value pairs without altering values or
// recursively treating checklist/verification mappings as integrations.
func orderedIntegration(node *yaml.Node) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return node
	}
	rank := func(key string) int {
		for i, known := range integrationKeyOrder {
			if key == known {
				return i
			}
		}
		return len(integrationKeyOrder)
	}
	var pairs [][2]*yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		pairs = append(pairs, [2]*yaml.Node{node.Content[i], node.Content[i+1]})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return rank(pairs[i][0].Value) < rank(pairs[j][0].Value)
	})
	copy := *node
	copy.Content = make([]*yaml.Node, 0, len(node.Content))
	for _, pair := range pairs {
		copy.Content = append(copy.Content, pair[:]...)
	}
	return &copy
}
