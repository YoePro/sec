package semantic

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildResolvedOperatorWithFailure lowers compiler-resolved operators and
// explicit failure routing, consuming safe integer-literal constants before
// operand materialization. It retains named-contract lowering boundaries and
// ordinary checked operations when no proof eliminates failure.
// Rules: rules/compiler/semantic_ir.md — §58;
// rules/errors/runtime_checks.md — §§5.1,7;
// rules/errors/errorhandling.md — §§9,11–13.
func (fb *functionBuilder) buildResolvedOperatorWithFailure(expr ast.Expression, arithmeticTry *sema.ResolvedTry, localTry *ast.TryExpression) (builtValue, error) {
	resolved, ok := fb.owner.analyzer.ResolvedOperatorOf(expr)
	if !ok {
		if value, handled, err := fb.buildShapedFloatSign(expr); handled {
			return value, err
		}
		return builtValue{}, fb.unsupported("unresolved or unsupported operator", expressionToken(expr))
	}
	// A value of a constrained named type is created only through a checked
	// conversion; an operator result in that type would need its own contract
	// check (contracts.md § 12(3)) and stays a named boundary until then.
	if len(resolved.ResultType.Contracts) > 0 && resolved.ResultType.Kind != sema.BoolType {
		return builtValue{}, fb.unsupported("operator result of constrained type "+resolved.ResultType.Name+" without a contract check", expressionToken(expr))
	}
	leftType, err := fb.owner.internType(resolved.LeftType)
	if err != nil {
		return builtValue{}, err
	}
	resultType, err := fb.owner.internType(resolved.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	if resolved.NegatedIntegerConstant != "" {
		// runtime_checks.md §§5.1,7: Sema proved this literal representable.
		// Materialize the exact value without rebuilding a dead checked edge
		// or evaluating a positive intermediate outside the signed range.
		value, valid := new(big.Int).SetString(resolved.NegatedIntegerConstant, 10)
		if !valid {
			return builtValue{}, fb.unsupported("invalid proven integer negation", expressionToken(expr))
		}
		return fb.result(Operation{Kind: OpConstInt, Integer: value, Location: locationFromExpression(expr)}, resultType), nil
	}
	var leftExpr, rightExpr ast.Expression
	switch expression := expr.(type) {
	case *ast.PrefixExpression:
		leftExpr = expression.Right
	case *ast.InfixExpression:
		leftExpr, rightExpr = expression.Left, expression.Right
	default:
		return builtValue{}, fb.unsupported("operator expression", expressionToken(expr))
	}
	// From package 14 a try operand is built in evaluation order like any
	// other: its failure edge leaves before the operator, and a later operand
	// is built on its success continuation (errorhandling.md § 11).
	if _, ok := leftExpr.(*ast.TryExpression); ok && fb.owner.maxPackage < 14 {
		return builtValue{}, fb.unsupported("try arithmetic", expressionToken(leftExpr))
	}
	if _, ok := rightExpr.(*ast.TryExpression); ok && fb.owner.maxPackage < 14 {
		return builtValue{}, fb.unsupported("try arithmetic", expressionToken(rightExpr))
	}
	left, err := fb.buildExpr(leftExpr, leftType)
	if err != nil {
		return builtValue{}, err
	}
	operands := []ValueID{left.id}
	if rightExpr != nil {
		rightType, err := fb.owner.internType(*resolved.RightType)
		if err != nil {
			return builtValue{}, err
		}
		right, err := fb.buildExpr(rightExpr, rightType)
		if err != nil {
			return builtValue{}, err
		}
		operands = append(operands, right.id)
	}
	loc := locationFromExpression(expr)
	if resolved.Kind == sema.ResolvedFloatUnaryPlus {
		// Floating unary plus preserves value, signed zero and NaN, so it is
		// the SSA identity (package20.md § 20).
		return left, nil
	}
	op := Operation{Operands: operands, Location: loc, Operator: operatorSpelling(expr)}
	if !semanticOperatorKind(resolved, &op) {
		return builtValue{}, fb.unsupported("operator "+string(resolved.Kind), expressionToken(expr))
	}
	if op.Kind == OpFloatCompare {
		op.FloatSemantics = resolvedFloatSemantics(fb.owner.module.Types, leftType, op)
	}
	if resolved.RuntimeCheck || resolved.ShiftCountProven {
		// A proven shift keeps the checked operation (Semantic IR has no
		// unchecked shift); its failure edge is dead.
		return fb.emitCheckedOperator(op, resultType, arithmeticTry, localTry, expr)
	}
	return fb.result(op, resultType), nil
}
