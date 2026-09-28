package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateCompileTimeIntegerArithmetic validates locally constant arithmetic.
// Division and remainder by zero remain blocking errors. Potential integer
// overflow emits S1023 as a warning and leaves the accepted expression on the
// ordinary checked-runtime path.
//
// Rules:
//   - rules/foundations/operators.md — "Checked integer arithmetic"
//   - rules/foundations/operators.md — "Compile-time overflow"
func (a *Analyzer) validateCompileTimeIntegerArithmetic(expr *ast.InfixExpression, resultType Type) bool {
	if expr == nil || !isBuiltinIntegerOperatorType(resultType) {
		return true
	}
	left, leftKnown := a.integerConstantValue(expr.Left)
	right, rightKnown := a.integerConstantValue(expr.Right)
	if !leftKnown || !rightKnown {
		return true
	}
	representation, _, ok := a.integerRepresentation(resultType)
	if !ok || representation.MinInteger == nil || representation.MaxInteger == nil {
		return true
	}
	if (expr.Operator == "/" || expr.Operator == "%") && right.Sign() == 0 {
		id := diagnostics.OperatorDivisionByZero
		operation := "division"
		help := "Use a non-zero divisor or guard the operation before evaluating it."
		if expr.Operator == "%" {
			id = diagnostics.OperatorRemainderByZero
			operation = "remainder"
			help = "Use a non-zero remainder divisor or guard the operation before evaluating it."
		}
		a.addErrorAtTokenWithMetadata(expr.Token, id, help, "constant integer %s by zero", operation)
		return false
	}
	value := new(big.Int)
	switch expr.Operator {
	case "+":
		value.Add(left, right)
	case "-":
		value.Sub(left, right)
	case "*":
		value.Mul(left, right)
	case "/":
		value.Quo(left, right)
	case "%":
		value.Rem(left, right)
	default:
		return true
	}
	divisionOverflow := (expr.Operator == "/" || expr.Operator == "%") &&
		representation.Kind == IntType && left.Cmp(representation.MinInteger) == 0 && right.Cmp(big.NewInt(-1)) == 0
	if !divisionOverflow && value.Cmp(representation.MinInteger) >= 0 && value.Cmp(representation.MaxInteger) <= 0 {
		return true
	}
	if a.refinementProvesIntegerArithmeticInRange(expr, resultType) {
		return true
	}
	a.addWarningAtTokenWithMetadata(
		expr.Token,
		diagnostics.OperatorIntegerOverflow,
		fmt.Sprintf("Ensure a dominating condition proves the operation remains within %s..%s, or use a wider integer type. The operation retains checked runtime semantics.", representation.MinInteger, representation.MaxInteger),
		"integer operation %s may overflow %s",
		expr.String(),
		typeDisplayName(resultType),
	)
	return true
}

// refinementProvesIntegerArithmeticInRange handles the narrow arithmetic
// consequence needed on a selected logical edge. For x - C with C >= 0, a
// dominating lower bound for x proves the subtraction cannot cross the
// representation minimum. Subtraction by a non-negative value cannot cross
// the upper bound.
//
// Rules:
//   - rules/foundations/operators.md — "Short-circuit evaluation"
//   - rules/foundations/operators.md — "Integer arithmetic"
func (a *Analyzer) refinementProvesIntegerArithmeticInRange(expr *ast.InfixExpression, resultType Type) bool {
	if expr == nil || expr.Operator != "-" {
		return false
	}
	right, ok := a.integerConstantValue(expr.Right)
	if !ok || right.Sign() < 0 {
		return false
	}
	representation, _, ok := a.integerRepresentation(resultType)
	if !ok || representation.MinInteger == nil {
		return false
	}
	lower, ok := a.activeIntegerLowerBound(expr.Left)
	if !ok {
		return false
	}
	return new(big.Int).Sub(lower, right).Cmp(representation.MinInteger) >= 0
}

func (a *Analyzer) activeIntegerLowerBound(target ast.Expression) (*big.Int, bool) {
	var strongest *big.Int
	excluded := map[string]bool{}
	for _, refinement := range a.activeConditionFacts {
		if refinement.epoch != a.arrayIndexMutationEpoch {
			continue
		}
		truth := refinement.fact.Kind != ConditionFactLogicalRHSFalse
		constraints := a.integerConstraintsForCondition(refinement.fact.Condition, target, truth)
		if constraints.lower != nil && (strongest == nil || constraints.lower.Cmp(strongest) > 0) {
			strongest = constraints.lower
		}
		for value := range constraints.excluded {
			excluded[value] = true
		}
	}
	for strongest != nil && excluded[strongest.String()] {
		strongest = new(big.Int).Add(strongest, big.NewInt(1))
	}
	return strongest, strongest != nil
}

type integerConditionConstraints struct {
	lower    *big.Int
	excluded map[string]bool
}

// integerConstraintsForCondition derives only consequences common to the
// selected logical edge. A true conjunction and a false disjunction both make
// every child condition known; the opposite combinations prove nothing.
func (a *Analyzer) integerConstraintsForCondition(condition, target ast.Expression, truth bool) integerConditionConstraints {
	comparison, ok := condition.(*ast.InfixExpression)
	if !ok || comparison == nil {
		return integerConditionConstraints{}
	}
	if (comparison.Operator == "&&" && truth) || (comparison.Operator == "||" && !truth) {
		left := a.integerConstraintsForCondition(comparison.Left, target, truth)
		right := a.integerConstraintsForCondition(comparison.Right, target, truth)
		if left.lower == nil || right.lower != nil && right.lower.Cmp(left.lower) > 0 {
			left.lower = right.lower
		}
		if left.excluded == nil {
			left.excluded = map[string]bool{}
		}
		for value := range right.excluded {
			left.excluded[value] = true
		}
		return left
	}
	if comparison.Operator == "&&" || comparison.Operator == "||" {
		return integerConditionConstraints{}
	}

	var bound *big.Int
	var targetOnLeft bool
	if a.sameResolvedExpression(comparison.Left, target) {
		bound, ok = a.integerConstantValue(comparison.Right)
		targetOnLeft = true
	} else if a.sameResolvedExpression(comparison.Right, target) {
		bound, ok = a.integerConstantValue(comparison.Left)
	} else {
		return integerConditionConstraints{}
	}
	if !ok {
		return integerConditionConstraints{}
	}

	result := integerConditionConstraints{}
	operator := comparison.Operator
	if !targetOnLeft {
		operator = reverseComparisonOperator(operator)
	}
	if !truth {
		operator = negateComparisonOperator(operator)
	}
	switch operator {
	case ">":
		result.lower = new(big.Int).Add(bound, big.NewInt(1))
	case ">=", "==":
		result.lower = new(big.Int).Set(bound)
	case "!=":
		result.excluded = map[string]bool{bound.String(): true}
	}
	return result
}

func reverseComparisonOperator(operator string) string {
	switch operator {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	default:
		return operator
	}
}

func negateComparisonOperator(operator string) string {
	switch operator {
	case "<":
		return ">="
	case "<=":
		return ">"
	case ">":
		return "<="
	case ">=":
		return "<"
	case "==":
		return "!="
	case "!=":
		return "=="
	default:
		return ""
	}
}
