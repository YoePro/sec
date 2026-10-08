package readiness

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
)

// CheckIntegerConversion proves a literal or the complete source bit domain
// fits the selected destination domain. Unknown narrowing and signedness loss
// cannot be implemented as truncation or reinterpretation while MD-012 leaves
// the public checked-conversion failure type undecided.
// Rules: rules/types/types.md — "Explicit conversions", "int and uint";
// rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md §4.
func CheckIntegerConversion(sourceBits int, sourceUnsigned bool, targetBits int, targetUnsigned bool, expression ast.Expression, knownValue *big.Int, token lexer.Token) error {
	sourceMin, sourceMax := integerDomain(sourceBits, sourceUnsigned)
	targetMin, targetMax := integerDomain(targetBits, targetUnsigned)
	if knownValue != nil {
		sourceMin, sourceMax = knownValue, knownValue
	} else if value, known := literalInteger(expression); known {
		sourceMin, sourceMax = value, value
	}
	if sourceMin.Cmp(targetMin) >= 0 && sourceMax.Cmp(targetMax) <= 0 {
		return nil
	}
	return &semantic.UnsupportedFeatureError{Feature: "runtime-checked integer conversion requires target-domain validation; public failure type is unresolved (MD-012)", Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
}

// integerDomain derives exact fixed/native bounds from the caller's resolved
// physical width; no architecture or compiler-host width participates.
// Rules: rules/types/types.md — Integer types; memory/layout.md — scalar layout.
func integerDomain(bits int, unsigned bool) (*big.Int, *big.Int) {
	if unsigned {
		return big.NewInt(0), new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
	}
	limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	return new(big.Int).Neg(limit), new(big.Int).Sub(limit, big.NewInt(1))
}

// literalInteger retains exact literal sign and magnitude for conversion proof.
// Rules: rules/types/types.md — Literal shaping versus conversion.
func literalInteger(expression ast.Expression) (*big.Int, bool) {
	switch expression := expression.(type) {
	case *ast.IntegerLiteral:
		if expression.Suffix() == "" || expression.Suffix() == "i" || expression.Suffix() == "u" {
			return ast.ParseIntegerLiteralLexeme(expression.Token.Lexeme)
		}
	case *ast.PrefixExpression:
		if expression.Operator == "-" {
			if value, ok := literalInteger(expression.Right); ok {
				return value.Neg(value), true
			}
		}
	}
	return nil, false
}
