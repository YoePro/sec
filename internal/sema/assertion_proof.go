package sema

import (
	"math/big"

	"sec/internal/ast"
)

// proveCondition reports a bool condition the compiler proves true on every
// path reaching it, from constants, integer type ranges narrowed by range
// contracts and by dominating comparisons against constants, `+`/`-` on those
// refined intervals, logical composition, an identical dominating condition
// fact, a dominating symbolic relation between the same two values that
// implies the asserted one (`a < b` proves `a <= b`, `a != b`, and `b > a`),
// and a chain of such relations through other values (`a < b && b <= c`
// proves `a < c`).
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
			if a.proveIntegerComparison(expr) || a.dominatingRelationImplies(expr) || a.transitiveRelationImplies(expr) {
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
	if value, constant := a.constantConditionIntegerValue(expr); constant {
		return value, value, true
	}
	if minimum, maximum, ok := a.bitwiseInterval(expr); ok {
		return minimum, maximum, true
	}
	if id, ok := expr.(*ast.Identifier); ok {
		if symbol, found := a.symbols[id.Value]; found && !symbol.Mutable && !symbol.Volatile && symbol.IntegerMinimum != nil && symbol.IntegerMaximum != nil {
			minimum, maximum := new(big.Int).Set(symbol.IntegerMinimum), new(big.Int).Set(symbol.IntegerMaximum)
			a.refineIntervalByDominatingFacts(expr, minimum, maximum)
			return minimum, maximum, true
		}
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
	if !ok {
		return nil, nil, false
	}
	minimum, maximum, ok := integerTypeInterval(typ)
	if !ok {
		return nil, nil, false
	}
	a.refineIntervalByDominatingFacts(expr, minimum, maximum)
	return minimum, maximum, true
}

// integerTypeInterval returns the values an integer type admits: its
// representable range narrowed by its range contracts.
func integerTypeInterval(typ Type) (*big.Int, *big.Int, bool) {
	if typ.MinInteger == nil || typ.MaxInteger == nil {
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
		// A logical right operand only proves that its left operand was
		// false, which is not a comparison fact that holds.
		if active.epoch == a.arrayIndexMutationEpoch && active.fact.Condition != nil && active.fact.Kind != ConditionFactLogicalRHSFalse {
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
			value, known := a.constantConditionIntegerValue(fact.Right)
			if !known {
				continue
			}
			constant = value
		} else if spelling, ok := relationOperandSpelling(fact.Right); ok && spelling == subject {
			value, known := a.constantConditionIntegerValue(fact.Left)
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
	// Range membership and disjunctions of bounded conditions, on both
	// logical edges (rules/foundations/operators.md — "Range membership").
	for _, active := range a.activeConditionFacts {
		if active.epoch != a.arrayIndexMutationEpoch || active.fact.Condition == nil {
			continue
		}
		low, high, ok := a.conditionBounds(active.fact.Condition, subject, active.fact.Kind != ConditionFactLogicalRHSFalse)
		if !ok {
			continue
		}
		if low != nil {
			raiseMin(minimum, low)
		}
		if high != nil {
			lowerMax(maximum, high)
		}
	}
}

// conditionBounds derives an interval for subject that holds when condition
// evaluates to truth: comparisons with constants, `in` and `not in` with a
// constant range, conjunctions, and disjunctions (whose bound is the hull of
// both sides). nil bounds are unbounded; ok=false means no information.
func (a *Analyzer) conditionBounds(condition ast.Expression, subject string, truth bool) (*big.Int, *big.Int, bool) {
	infix, ok := condition.(*ast.InfixExpression)
	if !ok || infix == nil {
		if prefix, isPrefix := condition.(*ast.PrefixExpression); isPrefix && prefix.Operator == "!" {
			return a.conditionBounds(prefix.Right, subject, !truth)
		}
		return nil, nil, false
	}
	switch infix.Operator {
	case "&&", "||":
		conjunction := (infix.Operator == "&&") == truth
		leftLow, leftHigh, leftOK := a.conditionBounds(infix.Left, subject, truth)
		rightLow, rightHigh, rightOK := a.conditionBounds(infix.Right, subject, truth)
		if conjunction {
			switch {
			case leftOK && rightOK:
				return tighterLow(leftLow, rightLow), tighterHigh(leftHigh, rightHigh), true
			case leftOK:
				return leftLow, leftHigh, true
			case rightOK:
				return rightLow, rightHigh, true
			}
			return nil, nil, false
		}
		if !leftOK || !rightOK {
			return nil, nil, false
		}
		return looserLow(leftLow, rightLow), looserHigh(leftHigh, rightHigh), true
	case "in", "not in":
		if spelling, pure := relationOperandSpelling(infix.Left); !pure || spelling != subject {
			return nil, nil, false
		}
		if (infix.Operator == "in") != truth {
			return nil, nil, false
		}
		bounds, isRange := infix.Right.(*ast.RangeExpression)
		if !isRange || bounds.Start == nil || bounds.End == nil {
			return nil, nil, false
		}
		low, lowKnown := a.scalarBoundConstant(bounds.Start)
		high, highKnown := a.scalarBoundConstant(bounds.End)
		if !lowKnown || !highKnown {
			return nil, nil, false
		}
		if bounds.Exclusive {
			high = new(big.Int).Sub(high, big.NewInt(1))
		}
		return low, high, true
	case "<", "<=", ">", ">=", "==":
		operator := infix.Operator
		var constant *big.Int
		if spelling, pure := relationOperandSpelling(infix.Left); pure && spelling == subject {
			value, known := a.scalarBoundConstant(infix.Right)
			if !known {
				return nil, nil, false
			}
			constant = value
		} else if spelling, pure := relationOperandSpelling(infix.Right); pure && spelling == subject {
			value, known := a.scalarBoundConstant(infix.Left)
			if !known {
				return nil, nil, false
			}
			constant = value
			operator = mirroredRelation(operator)
		} else {
			return nil, nil, false
		}
		if !truth {
			switch operator {
			case "<":
				operator = ">="
			case "<=":
				operator = ">"
			case ">":
				operator = "<="
			case ">=":
				operator = "<"
			default:
				return nil, nil, false
			}
		}
		one := big.NewInt(1)
		switch operator {
		case "<":
			return nil, new(big.Int).Sub(constant, one), true
		case "<=":
			return nil, constant, true
		case ">":
			return new(big.Int).Add(constant, one), nil, true
		case ">=":
			return constant, nil, true
		case "==":
			return constant, constant, true
		}
	}
	return nil, nil, false
}

func tighterLow(a, b *big.Int) *big.Int {
	if a == nil || b != nil && b.Cmp(a) > 0 {
		return b
	}
	return a
}

func tighterHigh(a, b *big.Int) *big.Int {
	if a == nil || b != nil && b.Cmp(a) < 0 {
		return b
	}
	return a
}

func looserLow(a, b *big.Int) *big.Int {
	if a == nil || b == nil {
		return nil
	}
	if b.Cmp(a) < 0 {
		return b
	}
	return a
}

func looserHigh(a, b *big.Int) *big.Int {
	if a == nil || b == nil {
		return nil
	}
	if b.Cmp(a) > 0 {
		return b
	}
	return a
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

// relationEdge is one dominating fact `from <= to` (strict: `from < to`)
// between two pure values.
type relationEdge struct {
	to     string
	strict bool
}

// transitiveRelationImplies reports that a chain of dominating comparisons
// through other pure values implies the asserted comparison. Every fact is
// normalized to `x < y` or `x <= y` edges (`==` gives both directions, `!=`
// gives none); the claim holds when a path of sufficient strictness connects
// its operands: `l < r` needs a path from l to r with a strict edge, `l <= r`
// any path, `l == r` paths in both directions, and `l != r` a strict path in
// either direction. Only facts no mutation has invalidated participate.
//
// Rules:
//   - rules/errors/panic.md — § 15.6(1), (3) "Assertion refinement", § 15.8(1) proof on every path
func (a *Analyzer) transitiveRelationImplies(claim *ast.InfixExpression) bool {
	left, leftPure := relationOperandSpelling(claim.Left)
	right, rightPure := relationOperandSpelling(claim.Right)
	if !leftPure || !rightPure || left == right {
		return false
	}
	edges := map[string][]relationEdge{}
	for _, fact := range a.activeComparisonFacts() {
		from, okFrom := relationOperandSpelling(fact.Left)
		to, okTo := relationOperandSpelling(fact.Right)
		if !okFrom || !okTo || from == to {
			continue
		}
		switch fact.Operator {
		case "<":
			edges[from] = append(edges[from], relationEdge{to: to, strict: true})
		case "<=":
			edges[from] = append(edges[from], relationEdge{to: to})
		case ">":
			edges[to] = append(edges[to], relationEdge{to: from, strict: true})
		case ">=":
			edges[to] = append(edges[to], relationEdge{to: from})
		case "==":
			edges[from] = append(edges[from], relationEdge{to: to})
			edges[to] = append(edges[to], relationEdge{to: from})
		}
	}
	if len(edges) == 0 {
		return false
	}
	switch claim.Operator {
	case "<":
		_, strict := relationPath(edges, left, right)
		return strict
	case "<=":
		reachable, _ := relationPath(edges, left, right)
		return reachable
	case ">":
		_, strict := relationPath(edges, right, left)
		return strict
	case ">=":
		reachable, _ := relationPath(edges, right, left)
		return reachable
	case "==":
		forward, _ := relationPath(edges, left, right)
		backward, _ := relationPath(edges, right, left)
		return forward && backward
	case "!=":
		_, forward := relationPath(edges, left, right)
		_, backward := relationPath(edges, right, left)
		return forward || backward
	}
	return false
}

// relationPath searches the relation graph from one value to another and
// reports whether any path exists and whether some path contains a strict
// edge.
func relationPath(edges map[string][]relationEdge, from string, to string) (reachable bool, strict bool) {
	type state struct {
		node   string
		strict bool
	}
	seen := map[state]bool{{node: from}: true}
	queue := []state{{node: from}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range edges[current.node] {
			next := state{node: edge.to, strict: current.strict || edge.strict}
			if next.node == to {
				reachable = true
				if next.strict {
					return true, true
				}
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return reachable, false
}

// recordIntegerBindingInterval snapshots the proven interval of an immutable
// integer initializer on its symbol, so later reads of the binding keep the
// proof (for example a checked conversion of int(value) where value: char).
// Symbols carry the fact through lexical scope copies; mutable and volatile
// bindings never get one.
// Rules: rules/types/types.md — "Explicit conversions" (proven conversions need no runtime check);
// rules/foundations/names_scopes_visibility.md — Lexical scopes.
func (a *Analyzer) recordIntegerBindingInterval(name string, expr ast.Expression) {
	symbol, ok := a.symbols[name]
	if !ok || symbol.Mutable || symbol.Volatile || !isIntegerType(symbol.Type) {
		return
	}
	minimum, maximum, known := a.integerInterval(expr)
	if !known {
		if call, isCall := expr.(*ast.CallExpression); isCall && len(call.Arguments) == 1 {
			if source, typed := a.expressionTypes[call.Arguments[0]]; typed {
				minimum, maximum, known = scalarIntegerDomain(source)
			}
		}
	}
	if !known {
		return
	}
	if typeMin, typeMax, ok := integerTypeInterval(symbol.Type); ok {
		if typeMin.Cmp(minimum) > 0 {
			minimum = typeMin
		}
		if typeMax.Cmp(maximum) < 0 {
			maximum = typeMax
		}
	}
	symbol.IntegerMinimum, symbol.IntegerMaximum = minimum, maximum
	a.symbols[name] = symbol
}

// bitwiseInterval bounds &, |, ^ and the result of a conversion call over
// non-negative operands: x & y is at most min(max x, max y), and x | y and
// x ^ y stay below the next power of two of the larger maximum.
func (a *Analyzer) bitwiseInterval(expr ast.Expression) (*big.Int, *big.Int, bool) {
	if call, ok := expr.(*ast.CallExpression); ok && len(call.Arguments) == 1 {
		target, typed := a.expressionTypes[call]
		if !typed || !isIntegerType(target) {
			return nil, nil, false
		}
		if _, checked := a.checkedConversions[call]; checked {
			return nil, nil, false
		}
		source, ok := a.expressionTypes[call.Arguments[0]]
		if !ok {
			return nil, nil, false
		}
		if isIntegerType(source) {
			// An infallible integer conversion preserves the value.
			if !a.isExplicitConversionExpression(call) {
				return nil, nil, false
			}
			return a.integerInterval(call.Arguments[0])
		}
		minimum, maximum, known := scalarIntegerDomain(source)
		if known {
			a.refineIntervalByDominatingFacts(call.Arguments[0], minimum, maximum)
		}
		return minimum, maximum, known
	}
	infix, ok := expr.(*ast.InfixExpression)
	if ok && infix.Operator == "<<" {
		// A non-negative value shifted left by a constant amount.
		shift, constant := a.integerConstantValue(infix.Right)
		valueMin, valueMax, known := a.integerInterval(infix.Left)
		if !constant || !known || shift.Sign() < 0 || shift.BitLen() > 16 || valueMin.Sign() < 0 {
			return nil, nil, false
		}
		amount := uint(shift.Int64())
		return new(big.Int).Lsh(valueMin, amount), new(big.Int).Lsh(valueMax, amount), true
	}
	if !ok || (infix.Operator != "&" && infix.Operator != "|" && infix.Operator != "^") {
		return nil, nil, false
	}
	if infix.Operator == "&" {
		// In two's complement, masking with a non-negative constant yields a
		// value in 0..mask whatever the other operand's sign.
		for _, side := range []ast.Expression{infix.Left, infix.Right} {
			if mask, constant := a.integerConstantValue(side); constant && mask.Sign() >= 0 {
				return big.NewInt(0), mask, true
			}
		}
	}
	leftMin, leftMax, leftKnown := a.integerInterval(infix.Left)
	rightMin, rightMax, rightKnown := a.integerInterval(infix.Right)
	if !leftKnown || !rightKnown || leftMin.Sign() < 0 || rightMin.Sign() < 0 {
		return nil, nil, false
	}
	if infix.Operator == "&" {
		maximum := leftMax
		if rightMax.Cmp(maximum) < 0 {
			maximum = rightMax
		}
		return big.NewInt(0), new(big.Int).Set(maximum), true
	}
	larger := leftMax
	if rightMax.Cmp(larger) > 0 {
		larger = rightMax
	}
	bound := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(larger.BitLen())), big.NewInt(1))
	return big.NewInt(0), bound, true
}

// scalarBoundConstant evaluates a constant bound of a range or comparison
// condition, including the code value of t- and r-suffixed literals and of
// single-quoted character literals, so conditions on char and rune values
// also bound their integer conversions.
func (a *Analyzer) scalarBoundConstant(expr ast.Expression) (*big.Int, bool) {
	switch literal := expr.(type) {
	case *ast.IntegerLiteral:
		if suffix := literal.Suffix(); suffix == "t" || suffix == "r" {
			lexeme, _ := ast.SplitNumericLiteralSuffix(literal.Token.Lexeme)
			return ast.ParseIntegerLiteralLexeme(lexeme)
		}
	case *ast.CharLiteral:
		if scalar, ok := characterLiteralScalar(literal); ok {
			return big.NewInt(int64(scalar)), true
		}
		return nil, false
	}
	return a.constantConditionIntegerValue(expr)
}
