// Package operators owns Sec's canonical expression-operator metadata.
package operators

import "sec/internal/lexer"

// Precedence is ordered from the weakest expression binding to the strongest,
// matching the comparison used by the Pratt parser.
//
// Rules:
//   - rules/foundations/operators.md — "Canonical precedence"
//   - rules/foundations/operators.md — Appendix A.2 "Create one precedence definition"
type Precedence uint8

const (
	Lowest Precedence = iota
	LogicalOr
	LogicalAnd
	BitwiseOr
	BitwiseXor
	BitwiseAnd
	Equality
	Comparison
	Shift
	Additive
	Multiplicative
	Prefix
	Postfix
	Member
)

// Associativity records the grouping contract attached to one precedence
// category. NonChainable operators are rejected when nested without an
// intervening boolean expression.
type Associativity string

const (
	Left             Associativity = "left"
	Right            Associativity = "right"
	NonChainable     Associativity = "non-chainable"
	LeftShortCircuit Associativity = "left-short-circuit"
	LeftChaining     Associativity = "left-chaining"
)

// Definition is one canonical precedence row. Spellings contains source
// operators or a descriptive postfix form when the syntax is not one token.
type Definition struct {
	Category      string
	Precedence    Precedence
	Associativity Associativity
	Spellings     []string
}

var definitions = []Definition{
	{Category: "logical-or", Precedence: LogicalOr, Associativity: LeftShortCircuit, Spellings: []string{"||"}},
	{Category: "logical-and", Precedence: LogicalAnd, Associativity: LeftShortCircuit, Spellings: []string{"&&"}},
	{Category: "bitwise-or", Precedence: BitwiseOr, Associativity: Left, Spellings: []string{"|"}},
	{Category: "bitwise-xor", Precedence: BitwiseXor, Associativity: Left, Spellings: []string{"^"}},
	{Category: "bitwise-and", Precedence: BitwiseAnd, Associativity: Left, Spellings: []string{"&"}},
	{Category: "equality-and-state-test", Precedence: Equality, Associativity: NonChainable, Spellings: []string{"==", "!=", "is", "is not"}},
	{Category: "ordered-comparison-and-membership", Precedence: Comparison, Associativity: NonChainable, Spellings: []string{"<", "<=", ">", ">=", "in", "not in"}},
	{Category: "shift", Precedence: Shift, Associativity: Left, Spellings: []string{"<<", ">>"}},
	{Category: "additive", Precedence: Additive, Associativity: Left, Spellings: []string{"+", "-"}},
	{Category: "multiplicative", Precedence: Multiplicative, Associativity: Left, Spellings: []string{"*", "/", "%", "x"}},
	{Category: "prefix", Precedence: Prefix, Associativity: Right, Spellings: []string{"unary +", "unary -", "!", "~"}},
	{Category: "postfix", Precedence: Postfix, Associativity: LeftChaining, Spellings: []string{"call ()", "index or slice []", "struct literal {}", "spread ..."}},
	{Category: "member", Precedence: Member, Associativity: LeftChaining, Spellings: []string{"."}},
}

// Definitions returns a detached copy of the canonical weak-to-strong table
// for tests, tooling, and documentation generation.
//
// Rules:
//   - rules/foundations/operators.md — "Canonical precedence"
//   - rules/foundations/operators.md — Appendix A.2 "Create one precedence definition"
func Definitions() []Definition {
	out := make([]Definition, len(definitions))
	for index, definition := range definitions {
		out[index] = definition
		out[index].Spellings = append([]string(nil), definition.Spellings...)
	}
	return out
}

// BinaryPrecedence resolves an ordinary or contextual binary spelling through
// the same table used by the parser. Prefix and postfix descriptive spellings
// are deliberately excluded.
//
// Rules:
//   - rules/foundations/operators.md — "Canonical precedence"
func BinaryPrecedence(spelling string) (Precedence, bool) {
	for _, definition := range definitions {
		if definition.Precedence > Multiplicative {
			continue
		}
		for _, candidate := range definition.Spellings {
			if candidate == spelling {
				return definition.Precedence, true
			}
		}
	}
	return Lowest, false
}

// TokenPrecedence maps hard lexer tokens and postfix delimiters into the
// canonical table. Contextual spellings such as x, is, and not in are resolved
// with BinaryPrecedence after the parser has established their grammar role.
//
// Rules:
//   - rules/foundations/operators.md — "Canonical precedence"
func TokenPrecedence(token lexer.TokenType) (Precedence, bool) {
	var spelling string
	switch token {
	case lexer.OR:
		spelling = "||"
	case lexer.AND:
		spelling = "&&"
	case lexer.BIT_OR:
		spelling = "|"
	case lexer.BIT_XOR:
		spelling = "^"
	case lexer.BIT_AND:
		spelling = "&"
	case lexer.EQ:
		spelling = "=="
	case lexer.NEQ:
		spelling = "!="
	case lexer.LT:
		spelling = "<"
	case lexer.LTE:
		spelling = "<="
	case lexer.GT:
		spelling = ">"
	case lexer.GTE:
		spelling = ">="
	case lexer.IN:
		spelling = "in"
	case lexer.SHIFT_LEFT:
		spelling = "<<"
	case lexer.SHIFT_RIGHT:
		spelling = ">>"
	case lexer.PLUS:
		spelling = "+"
	case lexer.MINUS:
		spelling = "-"
	case lexer.ASTERISK:
		spelling = "*"
	case lexer.SLASH:
		spelling = "/"
	case lexer.PERCENT:
		spelling = "%"
	case lexer.LPAREN, lexer.LBRACKET, lexer.LBRACE, lexer.SPREAD:
		return Postfix, true
	case lexer.DOT:
		return Member, true
	default:
		return Lowest, false
	}
	return BinaryPrecedence(spelling)
}
