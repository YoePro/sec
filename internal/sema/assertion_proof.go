package sema

import (
	"math/big"

	"sec/internal/ast"
)

// proveCondition reports a bool condition the compiler proves true on every
// path reaching it, from constants, integer type ranges narrowed by range
// contracts, logical composition, and an identical dominating condition fact
// that no mutation has invalidated. It never evaluates the condition; the
// condition's own effects are recorded independently.
//
// Rules:
//   - rules/errors/panic.md — § 15.6 "Assertion refinement", § 15.8 "Assertions in @noPanic"
//   - rules/errors/panic.md — § 21(6)–(7) proven sources contribute no panic effect
func (a *Analyzer) proveCondition(condition ast.Expression) bool {
	switch expr := condition.(type) {
	case *ast.BooleanLiteral:
		return expr.Value
	case *ast.InfixExpression:
		switch expr.Operator {
		case "&&":
			return a.proveCondition(expr.Left) && a.proveCondition(expr.Right)
		case "||":
			return a.proveCondition(expr.Left) || a.proveCondition(expr.Right)
		case "<", "<=", ">", ">=", "==", "!=":
			if a.proveIntegerComparison(expr) {
				return true
			}
		}
	}
	return a.dominatingConditionProves(condition)
}

// proveIntegerComparison compares the static intervals of two integer
// operands.
func (a *Analyzer) proveIntegerComparison(expr *ast.InfixExpression) bool {
	leftMin, leftMax, leftKnown := a.integerInterval(expr.Left)
	rightMin, rightMax, rightKnown := a.integerInterval(expr.Right)
	if !leftKnown || !rightKnown {
		return false
	}
	switch expr.Operator {
	case "<":
		return leftMax.Cmp(rightMin) < 0
	case "<=":
		return leftMax.Cmp(rightMin) <= 0
	case ">":
		return leftMin.Cmp(rightMax) > 0
	case ">=":
		return leftMin.Cmp(rightMax) >= 0
	case "==":
		return leftMin.Cmp(leftMax) == 0 && rightMin.Cmp(rightMax) == 0 && leftMin.Cmp(rightMin) == 0
	case "!=":
		return leftMax.Cmp(rightMin) < 0 || rightMax.Cmp(leftMin) < 0
	}
	return false
}

// integerInterval returns the static value interval of an integer operand:
// a compile-time constant, or the representable range of its resolved type
// narrowed by its range contracts.
func (a *Analyzer) integerInterval(expr ast.Expression) (*big.Int, *big.Int, bool) {
	if value, constant := a.integerConstantValue(expr); constant {
		return value, value, true
	}
	typ, ok := a.expressionTypes[expr]
	if !ok || typ.MinInteger == nil || typ.MaxInteger == nil {
		return nil, nil, false
	}
	minimum := new(big.Int).Set(typ.MinInteger)
	maximum := new(big.Int).Set(typ.MaxInteger)
	for _, contract := range typ.Contracts {
		rangeContract, isRange := contract.(RangeContract)
		if !isRange {
			continue
		}
		if rangeContract.Min != nil && rangeContract.Min.Cmp(minimum) > 0 {
			minimum.Set(rangeContract.Min)
		}
		if rangeContract.Max != nil {
			contractMaximum := new(big.Int).Set(rangeContract.Max)
			if rangeContract.Exclusive {
				contractMaximum.Sub(contractMaximum, big.NewInt(1))
			}
			if contractMaximum.Cmp(maximum) < 0 {
				maximum.Set(contractMaximum)
			}
		}
	}
	return minimum, maximum, true
}

// dominatingConditionProves reports an identical condition that an earlier
// successful assertion or dominating true branch established and that no
// later mutation has invalidated.
func (a *Analyzer) dominatingConditionProves(condition ast.Expression) bool {
	spelling := condition.String()
	for _, active := range a.activeConditionFacts {
		if active.epoch != a.arrayIndexMutationEpoch || active.fact.Condition == nil {
			continue
		}
		if active.fact.Condition.String() == spelling {
			return true
		}
	}
	return false
}
