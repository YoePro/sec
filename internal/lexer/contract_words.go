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
	// PatternContractWord is the compile-time pattern contract `regex`. It is
	// recognized only in contract position and is not part of the §7.3 reserved
	// contract-word inventory until that rulebook records it (MD-009).
	PatternContractWord
)

// regexContractWord is the contract spelling defined by rules/types/contracts.md
// "Applicability" and "String and collection contracts".
const regexContractWord = "regex"

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
	case regexContractWord:
		return PatternContractWord
	default:
		return NotContractWord
	}
}

// IsContractWord reports whether spelling belongs to the canonical reserved
// contextual contract-word inventory of §7.3. The pattern contract `regex` is
// deliberately excluded: contracts.md defines the contract, but its
// reservation is not recorded in lexical_structure.md (MD-009).
//
// Rules:
//   - rules/foundations/lexical_structure.md — §7.3 "Contract words"
func IsContractWord(spelling string) bool {
	switch ContractWordRoleOf(spelling) {
	case ValueContractWord, MarkerContractWord:
		return true
	default:
		return false
	}
}

// IsContractStartWord reports whether spelling begins a contract when it
// appears in contract position after a named type's base type. It includes
// the reserved §7.3 words and the contextual `regex` pattern contract.
//
// Rules:
//   - rules/foundations/grammar.md — "Type contracts"
//   - rules/types/contracts.md — "Applicability" and "String and collection contracts"
func IsContractStartWord(spelling string) bool {
	return ContractWordRoleOf(spelling) != NotContractWord
}

// ContractStartWords returns every contextual contract spelling recognized in
// contract position, including `regex`, for tooling completion. Use
// ContractWords for the reserved inventory.
//
// Rules:
//   - rules/foundations/grammar.md — "Type contracts"
//   - rules/types/contracts.md — "Applicability"
func ContractStartWords() []string {
	return append(ContractWords(), regexContractWord)
}
