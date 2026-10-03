package sema

import (
	"math/big"

	"sec/internal/ast"
)

// proveCondition reports a bool condition the compiler proves true on every
// path reaching it, from constants, integer type ranges narrowed by range
// contracts and by dominating comparisons against constants, `+`/`-` on those
// refined intervals, logical composition, an identical dominating condition
// fact, and a dominating symbolic relation between the same two values that
// implies the asserted one (`a < b` proves `a <= b`, `a != b`, and `b > a`).
// Only facts no mutation has invalidated are used. It never evaluates the condition; the
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
			if a.proveIntegerComparison(expr) || a.dominatingRelationImplies(expr) {
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
	if infix, ok := expr.(*ast.InfixExpression); ok && (infix.Operator == "+" || infix.Operator == "-") {
		leftMin, leftMax, leftKnown := a.integerInterval(infix.Left)
		rightMin, rightMax, rightKnown := a.integerInterval(infix.Right)
		if !leftKnown || !rightKnown {
			return nil, nil, false
		}
		if infix.Operator == "+" {
			return new(big.Int).Add(leftMin, rightMin), new(big.Int).Add(leftMax, rightMax), true
		}
		return new(big.Int).Sub(leftMin, rightMax), new(big.Int).Sub(leftMax, rightMin), true
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
	a.refineIntervalByDominatingFacts(expr, minimum, maximum)
	return minimum, maximum, true
}

// activeComparisonFacts returns the comparison conjuncts of every dominating
// condition fact that no mutation has invalidated.
func (a *Analyzer) activeComparisonFacts() []*ast.InfixExpression {
	facts := []*ast.InfixExpression{}
	var collect func(ast.Expression)
	collect = func(expression ast.Expression) {
		infix, ok := expression.(*ast.InfixExpression)
		if !ok {
			return
		}
		switch infix.Operator {
		case "&&":
			collect(infix.Left)
			collect(infix.Right)
		case "<", "<=", ">", ">=", "==", "!=":
			facts = append(facts, infix)
		}
	}
	for _, active := range a.activeConditionFacts {
		if active.epoch == a.arrayIndexMutationEpoch && active.fact.Condition != nil {
			collect(active.fact.Condition)
		}
	}
	return facts
}

// refineIntervalByDominatingFacts narrows [minimum, maximum] of a pure value
// by dominating comparisons of that same value against integer constants.
func (a *Analyzer) refineIntervalByDominatingFacts(expr ast.Expression, minimum, maximum *big.Int) {
	subject, pure := relationOperandSpelling(expr)
	if !pure {
		return
	}
	for _, fact := range a.activeComparisonFacts() {
		operator := fact.Operator
		var constant *big.Int
		if spelling, ok := relationOperandSpelling(fact.Left); ok && spelling == subject {
			value, known := a.integerConstantValue(fact.Right)
			if !known {
				continue
			}
			constant = value
		} else if spelling, ok := relationOperandSpelling(fact.Right); ok && spelling == subject {
			value, known := a.integerConstantValue(fact.Left)
			if !known {
				continue
			}
			constant = value
			operator = mirroredRelation(operator)
		} else {
			continue
		}
		switch operator {
		case "<":
			lowerMax(maximum, new(big.Int).Sub(constant, big.NewInt(1)))
		case "<=":
			lowerMax(maximum, constant)
		case ">":
			raiseMin(minimum, new(big.Int).Add(constant, big.NewInt(1)))
		case ">=":
			raiseMin(minimum, constant)
		case "==":
			raiseMin(minimum, constant)
			lowerMax(maximum, constant)
		}
	}
}

func lowerMax(maximum, candidate *big.Int) {
	if candidate.Cmp(maximum) < 0 {
		maximum.Set(candidate)
	}
}

func raiseMin(minimum, candidate *big.Int) {
	if candidate.Cmp(minimum) > 0 {
		minimum.Set(candidate)
	}
}

// relationOperandSpelling identifies a side-effect-free operand: a binding or
// a chain of member reads on one.
func relationOperandSpelling(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, true
	case *ast.MemberExpression:
		base, ok := relationOperandSpelling(expr.Object)
		if !ok || expr.Property == nil {
			return "", false
		}
		return base + "." + expr.Property.Value, true
	}
	return "", false
}

// dominatingRelationImplies reports that a dominating comparison between the
// same two pure values implies the asserted comparison: each relation is the
// set of orderings it admits, and the fact proves the claim when its set is a
// subset of the claim's.
func (a *Analyzer) dominatingRelationImplies(claim *ast.InfixExpression) bool {
	left, leftPure := relationOperandSpelling(claim.Left)
	right, rightPure := relationOperandSpelling(claim.Right)
	if !leftPure || !rightPure {
		return false
	}
	want := comparisonOrderings(claim.Operator)
	for _, fact := range a.activeComparisonFacts() {
		factLeft, okLeft := relationOperandSpelling(fact.Left)
		factRight, okRight := relationOperandSpelling(fact.Right)
		if !okLeft || !okRight {
			continue
		}
		operator := fact.Operator
		switch {
		case factLeft == left && factRight == right:
		case factLeft == right && factRight == left:
			operator = mirroredRelation(operator)
		default:
			continue
		}
		if have := comparisonOrderings(operator); have != 0 && have&^want == 0 {
			return true
		}
	}
	return false
}

// mirroredRelation swaps the operands of an ordered or equality comparison.
func mirroredRelation(operator string) string {
	if operator == "==" || operator == "!=" {
		return operator
	}
	return mirroredComparison(operator)
}

// comparisonOrderings encodes the orderings (less, equal, greater) a
// comparison admits as a bit set.
func comparisonOrderings(operator string) int {
	const less, equal, greater = 1, 2, 4
	switch operator {
	case "<":
		return less
	case "<=":
		return less | equal
	case ">":
		return greater
	case ">=":
		return greater | equal
	case "==":
		return equal
	case "!=":
		return less | greater
	}
	return 0
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

// assertConditionHelp is the mentor help for a non-bool assertion condition:
// Sec applies no truthiness conversion, so the help names the explicit test
// the programmer most likely meant for the condition's type.
//
// Rules:
//   - rules/errors/panic.md — § 15.2 "Condition typing", § 28(2), § 28(4)
func assertConditionHelp(condition ast.Expression, typ Type) string {
	subject := condition.String()
	switch {
	case isNumericType(typ):
		return "Sec has no truthiness conversion; write the comparison you mean, such as `assert " + subject + " != 0`."
	case typ.Kind == StringType:
		return "Sec has no truthiness conversion; write the test you mean, such as `assert " + subject + ".Len != 0`."
	case typ.Name == "Option" || (typ.Kind == UnionType && typ.Name == "Option"):
		return "Sec has no truthiness conversion; test the state explicitly, such as `assert " + subject + " is not None`."
	case typ.Kind == ResultType:
		return "A Result is not a condition; handle it with try, or test its state explicitly, such as `assert " + subject + ".ErrRef is None`."
	}
	return "The assertion condition must have type bool; Sec applies no truthiness conversion, so write an explicit comparison or state test."
}
