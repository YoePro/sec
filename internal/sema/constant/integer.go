// Package constant implements pure scalar constant operations without Analyzer
// state or dependencies on the parent sema package.
package constant

import (
	"math/big"
	"sec/internal/ast"
)

// Integer evaluates literal integer operator trees with arbitrary precision.
// Rules: rules/foundations/operators.md — Integer arithmetic, shifts;
// rules/compiler/compile_time_evaluation.md — §2(3).
func Integer(expr ast.Expression) (*big.Int, bool) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		switch expr.Suffix() {
		case "t", "r":
			return nil, false
		}
		return ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
	case *ast.PrefixExpression:
		if expr.Operator == "+" {
			return Integer(expr.Right)
		}
		if expr.Operator != "-" {
			return nil, false
		}
		value, ok := Integer(expr.Right)
		if !ok {
			return nil, false
		}
		return value.Neg(value), true
	case *ast.InfixExpression:
		left, ok := Integer(expr.Left)
		if !ok {
			return nil, false
		}

		right, ok := Integer(expr.Right)
		if !ok {
			return nil, false
		}

		return IntegerBinary(expr.Operator, left, right)
	default:
		return nil, false
	}
}

// IntegerBinary applies an integer operation without mutating either input.
// Rules: rules/foundations/operators.md — Integer arithmetic, shifts;
// rules/compiler/compile_time_evaluation.md — §2(3).
func IntegerBinary(operator string, left, right *big.Int) (*big.Int, bool) {
	value := new(big.Int)

	switch operator {
	case "+":
		return value.Add(left, right), true
	case "-":
		return value.Sub(left, right), true
	case "*":
		return value.Mul(left, right), true
	case "/":
		if right.Sign() == 0 {
			return nil, false
		}
		return value.Quo(left, right), true
	case "%":
		if right.Sign() == 0 {
			return nil, false
		}
		return value.Rem(left, right), true
	case "&":
		return value.And(left, right), true
	case "|":
		return value.Or(left, right), true
	case "^":
		return value.Xor(left, right), true
	case "<<":
		if !right.IsUint64() {
			return nil, false
		}
		return value.Lsh(left, uint(right.Uint64())), true
	case ">>":
		if !right.IsUint64() {
			return nil, false
		}
		return value.Rsh(left, uint(right.Uint64())), true
	default:
		return nil, false
	}
}
