package sema

import "sec/internal/ast"

// CollectionDefault is the allocation-free empty default of a compiler-known
// list: length zero, no live elements, and no allocation.
//
// Rule: rules/types/default_values.md — "List defaults".
const CollectionDefault DefaultKind = "collection"

// isDefaultableEmptyListType reports whether typ is a compiler-known list
// whose empty default exists independently of element defaultability.
//
// Rules:
//   - rules/types/default_values.md — "List defaults"
//   - rules/collections/collections.md — §13.3 empty list state
func isDefaultableEmptyListType(typ Type) bool {
	return isCompilerKnownListType(typ) && len(typ.ConstArgs) <= 1
}

// inferCollectionLiteral types the empty list literals `list[T] {}` and
// `list[T, Capacity] {}` as collection literals rather than struct
// construction. A literal synthesized for an omitted default carries no source
// type and adopts the expected list type of its storage site.
//
// Rules:
//   - rules/collections/collections.md — §13.3 canonical explicit empty forms
//   - rules/types/default_values.md — "List defaults"
func (a *Analyzer) inferCollectionLiteral(expr *ast.CollectionLiteral) (Type, expressionValue) {
	display := expressionValue{Display: expr.String()}
	if expr.Invalid {
		return Type{Kind: InvalidType}, display
	}
	if expr.Type == nil {
		expected, ok := a.expectedExpressionTypes[expr]
		if !ok || !isDefaultableEmptyListType(expected) {
			return Type{Kind: InvalidType}, display
		}
		return expected, expressionValue{Display: typeDisplayName(expected) + " {}"}
	}
	typ, ok := a.resolveType(expr.Type)
	if !ok || typ.Kind == InvalidType {
		return Type{Kind: InvalidType}, display
	}
	if !isDefaultableEmptyListType(typ) {
		a.addErrorAtToken(expr.Token, "%s {} is not a collection literal; only list[T] {} and list[T, Capacity] {} are defined", typeDisplayName(typ))
		return Type{Kind: InvalidType}, display
	}
	return typ, expressionValue{Display: typeDisplayName(typ) + " {}"}
}
