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
	// a reserved contract spelling (§7.3) lexed as an identifier and resolved
	// contextually in contract position (MD-009).
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
	"regex",
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
// contextual contract-word inventory of §7.3, including the pattern contract
// `regex`.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §7.3 "Contract words"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 10.1–10.5, 10.16
func IsContractWord(spelling string) bool {
	switch ContractWordRoleOf(spelling) {
	case ValueContractWord, MarkerContractWord, PatternContractWord:
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
// contract position for tooling completion. Since MD-009 reserved `regex`, it
// equals the reserved inventory of ContractWords.
//
// Rules:
//   - rules/foundations/grammar.md — "Type contracts"
//   - rules/types/contracts.md — "Applicability"
func ContractStartWords() []string {
	return ContractWords()
}
