package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildBooleanOperator lowers the bool-valued operators that Sema resolves
// outside the integer operator facts: `&&` and `||` through their
// short-circuit flow, `!` on bool, and range membership. It reports whether
// the expression was one of them.
func (fb *functionBuilder) buildBooleanOperator(expr ast.Expression) (builtValue, bool, error) {
	switch expression := expr.(type) {
	case *ast.InfixExpression:
		if flow, ok := fb.owner.analyzer.ResolvedLogicalFlowOf(expression); ok {
			value, err := fb.buildLogical(expression, flow)
			return value, true, err
		}
		if plan, ok := fb.owner.analyzer.ResolvedRangeMembershipOf(expression); ok {
			value, err := fb.buildRangeMembership(expression, plan)
			return value, true, err
		}
	case *ast.PrefixExpression:
		if resolved, ok := fb.owner.analyzer.ResolvedOperatorOf(expression); ok && resolved.Kind == sema.ResolvedBoolNot {
			boolType, err := fb.boolType()
			if err != nil {
				return builtValue{}, true, err
			}
			operand, err := fb.buildExpr(expression.Right, boolType)
			if err != nil {
				return builtValue{}, true, err
			}
			return fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{operand.id}, Operator: "!", Location: location(expression.Token)}, boolType), true, nil
		}
	}
	return builtValue{}, false, nil
}

// buildLogical lowers `left && right` and `left || right` with guaranteed
// left-to-right short-circuit evaluation. The left operand is evaluated first;
// on the short-circuit edge it is itself the result (false for `&&`, true for
// `||`), so the merge block receives it unchanged, and the right operand is
// evaluated only on the other edge. Sema's ResolvedLogicalFlow selects the
// constant forms: when the left operand is a proven short-circuit the right
// operand is never built, and when it is proven not to short-circuit the
// right operand is the result.
//
// Rules:
//   - rules/foundations/operators.md — "Short-circuit evaluation"
//   - rules/control-flow/flowcontrol_while.md — § 5; rules/control-flow/flowcontrol_if.md — §§ 4–6
//   - rules/compiler/semantic_ir.md — § 67(2) value-producing branches merge through typed block parameters
func (fb *functionBuilder) buildLogical(expr *ast.InfixExpression, flow sema.ResolvedLogicalFlow) (builtValue, error) {
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	left, err := fb.buildExpr(expr.Left, boolType)
	if err != nil {
		return builtValue{}, err
	}
	switch flow.RHSExecution {
	case sema.LogicalRHSNever:
		return left, nil
	case sema.LogicalRHSAlways:
		return fb.buildExpr(expr.Right, boolType)
	}
	loc := location(expr.Token)
	rhs := fb.newBlock()
	merge := fb.newBlock()
	result := fb.newValue(boolType, OwnershipImmediate, loc)
	merge.Parameters = []Value{result}
	shortCircuit := BranchTarget{Block: merge.ID, Arguments: []ValueID{left.id}}
	successors := []BranchTarget{{Block: rhs.ID}, shortCircuit}
	if !flow.EvaluateRHSWhenLeft {
		successors = []BranchTarget{shortCircuit, {Block: rhs.ID}}
	}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{left.id}, Successors: successors, Operator: flow.Operator, Location: loc})
	fb.current = rhs
	right, err := fb.buildExpr(expr.Right, boolType)
	if err != nil {
		return builtValue{}, err
	}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{right.id}}}, Operator: flow.Operator, Location: loc})
	fb.current = merge
	return builtValue{id: result.ID, typ: boolType}, nil
}

// buildRangeMembership lowers `value in lower..upper` (or `..<`) over a
// built-in integer type. The value is evaluated first and the present bounds
// left to right, each exactly once; the lower bound is inclusive and the
// upper bound inclusive or exclusive. When the lower test fails it is itself
// the false result, so only one merge is needed. `not in` negates the result
// after the test completes.
//
// Rules:
//   - rules/foundations/operators.md — "Membership expression", "Range membership", "Inclusive range", "Exclusive upper range"
func (fb *functionBuilder) buildRangeMembership(expr *ast.InfixExpression, plan sema.ResolvedRangeMembership) (builtValue, error) {
	rangeExpr, ok := expr.Right.(*ast.RangeExpression)
	if !ok || (!plan.HasStart && !plan.HasEnd) {
		return builtValue{}, fb.unsupported("range membership without bounds", expr.Token)
	}
	valueType, err := fb.owner.internType(plan.ValueType)
	if err != nil {
		return builtValue{}, err
	}
	if !isBuiltinIntegerType(fb.owner.module.Types, valueType) {
		return builtValue{}, fb.unsupported("range membership over "+plan.ValueType.Name, expr.Token)
	}
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	loc := location(expr.Token)
	value, err := fb.buildExpr(expr.Left, valueType)
	if err != nil {
		return builtValue{}, err
	}
	bound := func(bound ast.Expression) (builtValue, error) {
		built, err := fb.buildExpr(bound, valueType)
		if err == nil && built.typ != valueType {
			return builtValue{}, fb.unsupported("range bound of a different type", expressionToken(bound))
		}
		return built, err
	}
	var start, end builtValue
	if plan.HasStart {
		if start, err = bound(rangeExpr.Start); err != nil {
			return builtValue{}, err
		}
	}
	if plan.HasEnd {
		if end, err = bound(rangeExpr.End); err != nil {
			return builtValue{}, err
		}
	}
	var lower, upper builtValue
	if plan.HasStart {
		lower = fb.result(Operation{Kind: OpIntCompare, IntegerCompare: IntegerCompareGE, Operands: []ValueID{value.id, start.id}, Operator: expr.Operator, Location: loc}, boolType)
	}
	if plan.HasEnd {
		predicate := IntegerCompareLE
		if plan.Exclusive {
			predicate = IntegerCompareLT
		}
		upper = fb.result(Operation{Kind: OpIntCompare, IntegerCompare: predicate, Operands: []ValueID{value.id, end.id}, Operator: expr.Operator, Location: loc}, boolType)
	}
	result := lower
	switch {
	case !plan.HasStart:
		result = upper
	case plan.HasEnd:
		merge := fb.newBlock()
		contained := fb.newValue(boolType, OwnershipImmediate, loc)
		merge.Parameters = []Value{contained}
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{lower.id}, Successors: []BranchTarget{
			{Block: merge.ID, Arguments: []ValueID{upper.id}}, {Block: merge.ID, Arguments: []ValueID{lower.id}},
		}, Operator: expr.Operator, Location: loc})
		fb.current = merge
		result = builtValue{id: contained.ID, typ: boolType}
	}
	if plan.Negated {
		return fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{result.id}, Operator: expr.Operator, Location: loc}, boolType), nil
	}
	return result, nil
}

func (fb *functionBuilder) boolType() (TypeID, error) {
	return fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
}
