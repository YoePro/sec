package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validUnicodeScalarLiteral validates numeric t/r literals without weakening
// the scalar domain for privileged core source. Integer code points can be used
// to inspect invalid values before conversion; they are not rune literals.
// Rules: rules/foundations/lexical_structure.md — §12.7 "Numeric family suffixes"
// (t/r values exclude U+D800..U+DFFF);
// rules/types/types.md — "rune"; rules/library/core-library.md — §1.2 "Core-owned functionality";
// rules/tooling/diagnostics.md — §5(8) registered IDs and §7 diagnostic occurrences.
func (a *Analyzer) validUnicodeScalarLiteral(expr *ast.IntegerLiteral) bool {
	value, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
	if !ok || value.Sign() < 0 || value.Cmp(big.NewInt(0x10FFFF)) > 0 || (value.Cmp(big.NewInt(0xD800)) >= 0 && value.Cmp(big.NewInt(0xDFFF)) <= 0) {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.InvalidUnicodeScalarLiteral,
			"Use integer code points (for example 0xD800u..0xDFFFu) to test the surrogate range before conversion to a scalar type; a valid rune cannot be a surrogate, including in core code.",
			"value %s is not a valid Unicode scalar value; allowed values are U+0000..U+10FFFF excluding surrogates U+D800..U+DFFF", expr.Token.Lexeme)
		return false
	}
	return true
}

// charLiteralMaximum is the largest value of the 8-bit char domain.
var charLiteralMaximum = big.NewInt(255)

// validCharScalarLiteral validates a t-suffixed literal against char's
// complete 0..255 domain, independently of the Unicode scalar check used by
// r-suffixed literals: 256t and 300t are invalid even though U+0100 and
// U+012C are Unicode scalar values.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §12.7 "Numeric family suffixes"
//   - rules/types/types.md — "char"
//   - rules/corrections/applied/md043-char-rune-literal-correction-20261008.md — §§3.5, 6.3
func (a *Analyzer) validCharScalarLiteral(expr *ast.IntegerLiteral) bool {
	value, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
	if !ok || value.Sign() < 0 || value.Cmp(charLiteralMaximum) > 0 {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.CharLiteralOutOfRange,
			"Use a value in 0..255 for char, or the r suffix for a rune scalar value.",
			"value %s does not fit char; t-suffixed literals must be in 0..255", expr.Token.Lexeme)
		return false
	}
	return true
}

// characterLiteralScalar decodes the single Unicode scalar of a character
// literal after escape processing.
func characterLiteralScalar(literal *ast.CharLiteral) (rune, bool) {
	decoded, ok := lexer.DecodeCharacterLiteral(literal.Token.Lexeme)
	if !ok {
		return 0, false
	}
	return []rune(decoded)[0], true
}

// shapeCharacterLiteral applies contextual literal shaping to a single-quoted
// literal: it is rune by default, and an explicitly expected char shapes it
// only when its decoded scalar is in 0..255. Shaping is not a rune-to-char
// conversion of a typed expression. It reports handled=false for any other
// expression or context; an out-of-range scalar yields S1138 and InvalidType.
//
// Rules:
//   - rules/types/types.md — "char", "rune", "Character literal"
//   - rules/foundations/lexical_structure.md — §13 "Character literals"
//   - rules/corrections/applied/md043-char-rune-literal-correction-20261008.md — §§3.1–3.3
func (a *Analyzer) shapeCharacterLiteral(expr ast.Expression, context Type) (Type, bool) {
	literal, ok := expr.(*ast.CharLiteral)
	if !ok || (context.Kind != CharType && context.Kind != RuneType) {
		return Type{}, false
	}
	scalar, valid := characterLiteralScalar(literal)
	if !valid {
		return Type{}, false
	}
	if context.Kind == RuneType {
		typ := Type{Name: "rune", Kind: RuneType}
		a.expressionTypes[expr] = typ
		return typ, true
	}
	if scalar > 255 {
		a.addErrorAtTokenWithMetadata(literal.Token, diagnostics.CharLiteralOutOfRange,
			fmt.Sprintf("Declare the value as rune, or choose a character whose code point is in 0..255; U+%04X does not fit the 8-bit char.", scalar),
			"character literal %s does not fit char; char holds values 0..255", literal.Token.Lexeme)
		a.expressionTypes[expr] = Type{Kind: InvalidType}
		return Type{Kind: InvalidType}, true
	}
	typ := Type{Name: "char", Kind: CharType}
	a.expressionTypes[expr] = typ
	return typ, true
}
