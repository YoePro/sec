package cst

import (
	"reflect"
	"sort"

	"sec/internal/ast"
)

// applyBindingExtent keeps multiline initializer expressions outside single-
// line declaration tables, including unparenthesized operator continuations.
// Delimiter completion remains owned by the lossless hierarchy.
// Rules: rules/tooling/formatter.md — § 9(1)–(2), (8), § 29(2), (4).
func (d *Document) applyBindingExtent(node *ast.LetStatement, indexes map[concreteTokenKey]int) {
	if node.Name == nil || node.Value == nil {
		return
	}
	end := continuationExpressionTokenEnd(reflect.ValueOf(node.Value))
	last := sort.Search(len(d.Elements), func(i int) bool { return d.Elements[i].Span.End >= end })
	if last >= len(d.Elements) || d.Elements[last].Token.Line <= node.Name.Token.Line {
		return
	}
	if name, ok := indexes[keyForToken(node.Name.Token)]; ok && !d.Elements[name].HasRole(BindingMultiline) {
		d.Elements[name].Roles = append(d.Elements[name].Roles, BindingMultiline)
	}
}
