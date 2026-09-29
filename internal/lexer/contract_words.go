package lexer

// ContractWordRole describes the grammar role of a compiler-known contract
// spelling while allowing the spelling itself to remain an IDENT token.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §7.3 "Contract words"
//   - rules/foundations/grammar.md — "Type contracts"
type ContractWordRole uint8

const (
	NotContractWord ContractWordRole = iota
	ValueContractWord
	MarkerContractWord
)

var contractWords = []string{
	"multipleOf",
	"minLen",
	"maxLen",
	"exactLen",
	"notEmpty",
	"unique",
	"finite",
	"odd",
	"even",
}

// ContractWords returns the frontend-supported contract-word inventory in
// grammar presentation order. The returned slice is independent of lexer
// state.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §7.3 "Contract words"
//   - rules/foundations/grammar.md — "Type contracts"
func ContractWords() []string {
	words := make([]string, len(contractWords))
	copy(words, contractWords)
	return words
}

// ContractWordRoleOf classifies a contextual contract spelling. Contract
// words deliberately remain identifiers at the lexical token boundary.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §7.3 "Contract words"
//   - rules/foundations/grammar.md — "Type contracts"
func ContractWordRoleOf(spelling string) ContractWordRole {
	switch spelling {
	case "multipleOf", "minLen", "maxLen", "exactLen":
		return ValueContractWord
	case "notEmpty", "unique", "finite", "odd", "even":
		return MarkerContractWord
	default:
		return NotContractWord
	}
}

// IsContractWord reports whether spelling belongs to the canonical contextual
// contract-word inventory.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §7.3 "Contract words"
func IsContractWord(spelling string) bool {
	return ContractWordRoleOf(spelling) != NotContractWord
}
