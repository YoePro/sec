package lexer

// ReservedDeclarationNameKind describes why a source spelling cannot introduce
// a user declaration. Contextual spellings which the rulebook explicitly keeps
// available, notably set, x, not, init, arena, and sec, are not classified.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§7–9
//   - rules/foundations/names_scopes_visibility.md — §9 "Reserved language names"
type ReservedDeclarationNameKind uint8

const (
	NotReservedDeclarationName ReservedDeclarationNameKind = iota
	HardKeywordDeclarationName
	ContextReservedDeclarationName
	ContractDeclarationName
	CompilerKnownTypeDeclarationName
	GrammarSymbolDeclarationName
)

var compilerKnownLowercaseTypeNames = map[string]bool{
	"any": true, "bool": true, "byte": true, "char": true, "rune": true,
	"string": true, "void": true,
	"int": true, "int8": true, "int16": true, "int32": true,
	"int64": true, "int128": true, "int256": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true,
	"uint64": true, "uint128": true, "uint256": true,
	"float": true, "float32": true, "float64": true,
	"decimal": true, "decimal128": true,
	"date": true, "time": true, "datetime": true, "duration": true,
	"bit": true, "register": true,
	"list": true, "map": true,
	"vector": true, "matrix": true, "tensor": true, "tensor_view": true,
}

// ReservedDeclarationNameKindOf returns the canonical reservation class for a
// declaration spelling. Hard keywords participate even though the parser will
// normally reject them before an AST declaration can be formed.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§6.3–9
//   - rules/foundations/names_scopes_visibility.md — §9 "Reserved language names"
func ReservedDeclarationNameKindOf(spelling string) ReservedDeclarationNameKind {
	if spelling == "_" {
		return GrammarSymbolDeclarationName
	}
	if lookupIdent(spelling) != IDENT {
		return HardKeywordDeclarationName
	}
	switch spelling {
	case "task", "thread", "process":
		return ContextReservedDeclarationName
	}
	if IsContractWord(spelling) {
		return ContractDeclarationName
	}
	if compilerKnownLowercaseTypeNames[spelling] {
		return CompilerKnownTypeDeclarationName
	}
	return NotReservedDeclarationName
}

// IsReservedDeclarationName reports whether spelling is forbidden in a user
// declaration namespace. The contextual collection/accessor spelling set is
// intentionally false.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§7–9
func IsReservedDeclarationName(spelling string) bool {
	return ReservedDeclarationNameKindOf(spelling) != NotReservedDeclarationName
}
