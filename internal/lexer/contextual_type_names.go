package lexer

// ContextualTypeNameRole describes the grammar family of a compiler-known
// lowercase type constructor while allowing the spelling to remain an IDENT
// token outside a parser-established type position.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §8 "Compiler-known type names"
//   - rules/foundations/grammar.md — "Collection and shaped types"
type ContextualTypeNameRole uint8

const (
	NotContextualTypeName ContextualTypeNameRole = iota
	CollectionTypeName
	ShapedTypeName
)

var contextualCollectionShapedTypeNames = []string{
	"list",
	"map",
	"set",
	"vector",
	"matrix",
	"tensor",
	"tensor_view",
}

// ContextualCollectionShapedTypeNames returns the canonical lowercase
// collection and shaped type-name inventory in grammar presentation order.
// The returned slice is independent of lexer state.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§8–9
//   - rules/foundations/grammar.md — "Collection and shaped types"
func ContextualCollectionShapedTypeNames() []string {
	names := make([]string, len(contextualCollectionShapedTypeNames))
	copy(names, contextualCollectionShapedTypeNames)
	return names
}

// ContextualTypeNameRoleOf classifies a lowercase contextual type spelling.
// These names deliberately remain identifiers at the lexical token boundary;
// the parser assigns their type role only in a type-reference context.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§8–9
//   - rules/foundations/grammar.md — "Collection and shaped types"
func ContextualTypeNameRoleOf(spelling string) ContextualTypeNameRole {
	switch spelling {
	case "list", "map", "set":
		return CollectionTypeName
	case "vector", "matrix", "tensor", "tensor_view":
		return ShapedTypeName
	default:
		return NotContextualTypeName
	}
}

// IsContextualCollectionShapedTypeName reports whether spelling belongs to the
// canonical lowercase collection and shaped type-name inventory.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§8–9
func IsContextualCollectionShapedTypeName(spelling string) bool {
	return ContextualTypeNameRoleOf(spelling) != NotContextualTypeName
}

// ContextualTypeNameArgumentCount returns the number of leading type
// arguments required before the constructor's constant arguments. The second
// result is false for spellings outside the contextual inventory.
//
// Rules:
//   - rules/foundations/grammar.md — "Collection and shaped types"
func ContextualTypeNameArgumentCount(spelling string) (int, bool) {
	if !IsContextualCollectionShapedTypeName(spelling) {
		return 0, false
	}
	if spelling == "map" {
		return 2, true
	}
	return 1, true
}
