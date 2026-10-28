package sema

import (
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// decimalConstantValue evaluates expr exactly when it is a compile-time
// known decimal value: a decimal or integer literal, a sign, an immutable
// binding with a recorded decimal constant, or an exact +, - or * of such
// values. It never rounds and never evaluates a division.
// Rules: rules/foundations/operators.md — "Decimal arithmetic";
// rules/types/types.md — "Decimal scale".
func (a *Analyzer) decimalConstantValue(expr ast.Expression) (*big.Rat, bool) {
	switch node := expr.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral:
		literal, ok := decimalLiteralValue(node)
		if !ok {
			return nil, false
		}
		return decimalValueRat(literal), true
	case *ast.Identifier:
		symbol, ok := a.symbols[node.Value]
		if !ok || symbol.Mutable || symbol.Volatile || symbol.DecimalConstant == nil {
			return nil, false
		}
		return new(big.Rat).Set(symbol.DecimalConstant), true
	case *ast.PrefixExpression:
		value, ok := a.decimalConstantValue(node.Right)
		if !ok || node.Operator != "-" && node.Operator != "+" {
			return nil, false
		}
		if node.Operator == "-" {
			value.Neg(value)
		}
		return value, true
	case *ast.InfixExpression:
		left, leftOK := a.decimalConstantValue(node.Left)
		right, rightOK := a.decimalConstantValue(node.Right)
		if !leftOK || !rightOK {
			return nil, false
		}
		switch node.Operator {
		case "+":
			return left.Add(left, right), true
		case "-":
			return left.Sub(left, right), true
		case "*":
			return left.Mul(left, right), true
		}
	}
	return nil, false
}

// recordDecimalBindingConstant remembers the exact value of an immutable
// decimal binding so a later provably zero divisor is recognized.
// Rules: rules/foundations/operators.md — "Decimal division and remainder by zero".
func (a *Analyzer) recordDecimalBindingConstant(name string, expr ast.Expression) {
	symbol, ok := a.symbols[name]
	if !ok || symbol.Mutable || symbol.Volatile || dereferenceType(symbol.Type).Kind != DecimalType || hasUnitSemantics(symbol.Type) {
		return
	}
	value, ok := a.decimalConstantValue(expr)
	if !ok {
		return
	}
	symbol.DecimalConstant = value
	a.symbols[name] = symbol
}

// validateDecimalZeroDivisor rejects decimal division or remainder whose
// divisor the compiler proves to be zero. It is a compile-time error even
// under try, so compilation stops before code generation; a divisor that is
// not provably zero keeps its runtime check.
//
// Rules:
//   - rules/foundations/operators.md — "Decimal division and remainder by zero"
//   - rules/errors/runtime_checks.md — "Division and remainder"
//   - rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 6(1), 6(4)–6(5)
func (a *Analyzer) validateDecimalZeroDivisor(expr *ast.InfixExpression) bool {
	return a.validateDecimalDivisor(expr.Operator, expr.Right, expr.Token)
}

// validateDecimalDivisor applies the provably-zero rule to the divisor of a
// decimal `/`, `%`, `/=` or `%=`; other operators are accepted unchanged.
// Rules: rules/foundations/operators.md — "Decimal division and remainder by zero".
func (a *Analyzer) validateDecimalDivisor(operator string, divisorExpr ast.Expression, token lexer.Token) bool {
	operator = strings.TrimSuffix(operator, "=")
	if operator != "/" && operator != "%" {
		return true
	}
	divisor, known := a.decimalConstantValue(divisorExpr)
	if !known || divisor.Sign() != 0 {
		return true
	}
	id, operation := diagnostics.OperatorDivisionByZero, "division"
	if operator == "%" {
		id, operation = diagnostics.OperatorRemainderByZero, "remainder"
	}
	a.addErrorAtTokenWithMetadata(token, id,
		"The divisor "+divisorExpr.String()+" is provably zero, which is a compile-time error even under try; use a non-zero divisor or guard the operation.",
		"decimal %s by zero", operation)
	return false
}

// decimalValueRat is the exact value of a decimal literal.
func decimalValueRat(value DecimalValue) *big.Rat {
	if value.Coefficient == nil {
		return new(big.Rat)
	}
	scale := int64(value.Scale)
	power := new(big.Int).Exp(big.NewInt(10), big.NewInt(max(scale, -scale)), nil)
	if scale >= 0 {
		return new(big.Rat).SetFrac(value.Coefficient, power)
	}
	return new(big.Rat).SetInt(new(big.Int).Mul(value.Coefficient, power))
}
