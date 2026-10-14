package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildBuiltinBooleanEquality dispatches only Sema-resolved builtin boolean
// equality. It uses existing typed CFG/constant operations, so it needs no
// aggregate-package capability or additional Sec dialect schema.
// Rules: rules/foundations/operators.md — "Boolean equality";
// rules/compiler/semantic_ir.md — §67(2) (typed value-producing merges).
func (fb *functionBuilder) buildBuiltinBooleanEquality(expr ast.Expression) (builtValue, bool, error) {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return builtValue{}, false, nil
	}
	plan, ok := fb.owner.analyzer.ResolvedOperatorOf(infix)
	if !ok || (plan.Kind != sema.ResolvedBoolCompareEQ && plan.Kind != sema.ResolvedBoolCompareNE) {
		return builtValue{}, false, nil
	}
	value, err := fb.buildBooleanEquality(infix, plan)
	return value, true, err
}

// buildBooleanEquality evaluates both operands once, left to right, then uses
// typed branches to select right or its complement. Operand evaluation is eager
// even when the left operand is a constant. No arithmetic failure or integer
// representation is introduced, and no new Sec dialect operation is required.
// Rules: rules/foundations/operators.md — "Boolean equality", "Evaluation order";
// rules/compiler/semantic_ir.md — §67(2) (typed value-producing merges).
func (fb *functionBuilder) buildBooleanEquality(expr *ast.InfixExpression, plan sema.ResolvedOperator) (builtValue, error) {
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	left, err := fb.buildExpr(expr.Left, boolType)
	if err != nil {
		return builtValue{}, err
	}
	right, err := fb.buildExpr(expr.Right, boolType)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(expr.Token)
	invert := fb.newBlock()
	merge := fb.newBlock()
	result := fb.newValue(boolType, OwnershipImmediate, loc)
	merge.Parameters = []Value{result}
	direct := BranchTarget{Block: merge.ID, Arguments: []ValueID{right.id}}
	inverted := BranchTarget{Block: invert.ID}
	successors := []BranchTarget{direct, inverted}
	if plan.Kind == sema.ResolvedBoolCompareNE {
		successors = []BranchTarget{inverted, direct}
	}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{left.id}, Successors: successors, Operator: expr.Operator, Location: loc})
	fb.current = invert
	falseLiteral, trueLiteral := false, true
	falseValue := fb.result(Operation{Kind: OpConstBool, Bool: &falseLiteral, Location: loc}, boolType)
	trueValue := fb.result(Operation{Kind: OpConstBool, Bool: &trueLiteral, Location: loc}, boolType)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{right.id}, Successors: []BranchTarget{
		{Block: merge.ID, Arguments: []ValueID{falseValue.id}},
		{Block: merge.ID, Arguments: []ValueID{trueValue.id}},
	}, Operator: expr.Operator, Location: loc})
	fb.current = merge
	return builtValue{id: result.ID, typ: boolType}, nil
}
