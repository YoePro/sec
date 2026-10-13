package project

import (
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

// manifestKeyLine locates actual declarations through parsed key components,
// including inline/dotted keys, without interpreting comments or string text.
// Rules: rules/projects/projects.md — §§5(2),48(2),49(5).
func manifestKeyLine(text string, wanted []string) int {
	var parser unstable.Parser
	parser.Reset([]byte(text))
	var section []string
	for parser.NextExpression() {
		node := parser.Expression()
		switch node.Kind {
		case unstable.Table, unstable.ArrayTable:
			section = manifestSyntaxKey(node)
			if slices.Equal(section, wanted) {
				return strings.Count(text[:node.Child().Raw.Offset], "\n") + 1
			}
		case unstable.KeyValue:
			if line := manifestInlineKeyLine(text, node, section, wanted); line != 0 {
				return line
			}
		}
	}
	return 0
}

// manifestInlineKeyLine descends only actual inline-table assignments.
// Rules: rules/projects/projects.md — §48(2),§49(5).
func manifestInlineKeyLine(text string, node *unstable.Node, parent, wanted []string) int {
	path := append(slices.Clone(parent), manifestSyntaxKey(node)...)
	if slices.Equal(path, wanted) {
		key := node.Key()
		key.Next()
		return strings.Count(text[:key.Node().Raw.Offset], "\n") + 1
	}
	value := node.Value()
	if value.Kind == unstable.InlineTable && len(path) < len(wanted) && slices.Equal(path, wanted[:len(path)]) {
		children := value.Children()
		for children.Next() {
			if line := manifestInlineKeyLine(text, children.Node(), path, wanted); line != 0 {
				return line
			}
		}
	}
	return 0
}
