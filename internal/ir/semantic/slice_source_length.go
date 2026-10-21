package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// lowerSliceLength consumes the resolved core operation and evaluates its
// source once. Length observes the reference's validity without copying its
// holder, reading elements or converting its unsigned count to a signed type.
// Rules: rules/compiler/semantic_ir.md — §126.14;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45,47–49.
func (fb *functionBuilder) lowerSliceLength(expr ast.Expression, plan sema.ResolvedSliceLengthPlan) (builtValue, error) {
	var source ast.Expression
	switch node := expr.(type) {
	case *ast.MemberExpression:
		if plan.CompilerKnownID != "CKM-LEN-SLICE" {
			return builtValue{}, fmt.Errorf("slice length property has incompatible core identity")
		}
		source = node.Object
	case *ast.CallExpression:
		if plan.CompilerKnownID != "CKF-LEN" || len(node.Arguments) != 1 {
			return builtValue{}, fmt.Errorf("slice length call has incompatible core identity or arity")
		}
		source = node.Arguments[0]
	default:
		return builtValue{}, fmt.Errorf("slice length plan has incompatible source syntax")
	}
	value, facts, err := fb.sliceSourceValue(source)
	if err != nil {
		return builtValue{}, err
	}
	guard, err := fb.validateReference(value, facts, location(expressionToken(expr)))
	if err != nil {
		return builtValue{}, err
	}
	typ, err := fb.owner.internType(plan.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	result := fb.result(Operation{Kind: OpSliceLength, Operands: []ValueID{value.id}, ReferenceGuard: guard, Location: location(expressionToken(expr))}, typ)
	// An explicit inline borrow has no named holder to end later. End that
	// temporary after the observation, retaining the independent uint result.
	// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,63.
	if _, temporary := source.(*ast.RefExpression); temporary {
		if _, err := fb.discardSliceHolder(value, expressionToken(source)); err != nil {
			return builtValue{}, err
		}
	}
	return result, nil
}
