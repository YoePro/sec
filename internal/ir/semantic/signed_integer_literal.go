package semantic

import (
	"math/big"

	"sec/internal/ast"
)

// buildSignedIntegerLiteral materializes the complete signed literal in its
// Sema-validated contextual destination. In particular, the minimum signed
// value must not become an unrepresentable positive constant followed by a
// runtime negation. Runtime operators and nonliteral expressions retain their
// canonical checked operation plans.
// Rules: rules/types/types.md — "Context shaping", "int and uint";
// rules/compiler/semantic_ir.md — §11 "Constants";
// rules/memory/layout.md — §18(1–3).
func (fb *functionBuilder) buildSignedIntegerLiteral(expr ast.Expression, typeID TypeID) (builtValue, bool) {
	prefix, ok := expr.(*ast.PrefixExpression)
	if !ok || prefix.Operator != "+" && prefix.Operator != "-" {
		return builtValue{}, false
	}
	carrier, found := fb.owner.module.Types.Lookup(typeID)
	for found && carrier.Kind == TypeNamed {
		carrier, found = fb.owner.module.Types.Lookup(carrier.Base)
	}
	if !found || carrier.Kind != TypeInt && carrier.Kind != TypeUint {
		return builtValue{}, false
	}
	sign := 1
	var operand ast.Expression = prefix
	for {
		nested, ok := operand.(*ast.PrefixExpression)
		if !ok {
			break
		}
		if nested.Operator != "+" && nested.Operator != "-" {
			return builtValue{}, false
		}
		if nested.Operator == "-" {
			sign = -sign
		}
		operand = nested.Right
	}
	literal, ok := operand.(*ast.IntegerLiteral)
	if !ok {
		return builtValue{}, false
	}
	value, ok := ast.ParseIntegerLiteralLexeme(literal.Token.Lexeme)
	if !ok {
		return builtValue{}, false
	}
	value = new(big.Int).Set(value)
	if sign < 0 {
		value.Neg(value)
	}
	return fb.result(Operation{Kind: OpConstInt, Integer: value, Location: locationFromExpression(expr)}, typeID), true
}
