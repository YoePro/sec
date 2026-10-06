package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
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
