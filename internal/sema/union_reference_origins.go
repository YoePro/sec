package sema

import (
	"sec/internal/ast"
	"sort"
)

// recordUnionReferenceOrigins records direct and nested payload dependencies
// while their lexical sources are available. Variant-qualified paths preserve
// distinct payload slots through joins and returned-origin substitution. A
// container is not confused with storage of one of its referents.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/memory/lifetime_analysis.md §§14,28;
// rules/memory/borrowing.md §§12–14.
func (a *Analyzer) recordUnionReferenceOrigins(expr ast.Expression, plan ResolvedUnionConstruction) {
	delete(a.expressionReferenceOrigins, expr)
	contents := map[string]localReferenceOrigin{}
	for _, operand := range plan.PayloadPreparation {
		if operand.Defaulted {
			continue
		}
		path := "." + plan.VariantName
		if plan.Kind == ResolvedUnionVariantFields {
			path += "." + operand.FieldName
		}
		if direct, known := a.directReferenceOrigin(operand.Source); known {
			contents[path] = cloneLocalReferenceOrigin(direct)
		}
		prefixContainedOrigins(path, a.referenceContentsForBinding(operand.Source), contents)
	}
	if len(contents) == 0 {
		return
	}
	origin := localReferenceOrigin{Contained: contents}
	keys := make([]string, 0, len(contents))
	for key := range contents {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		child := contents[key]
		if child.Local && !origin.Local {
			origin.Local, origin.Name, origin.Token = true, child.Name, child.Token
		}
		if child.MatchScoped && !origin.MatchScoped {
			origin.MatchScoped = true
			if origin.Name == "" {
				origin.Name, origin.Token = child.Name, child.Token
			}
		}
		origin.Unknown = origin.Unknown || child.Unknown
		origin.Ambiguous = origin.Ambiguous || child.Ambiguous
	}
	a.expressionReferenceOrigins[expr] = cloneLocalReferenceOrigin(origin)
}
