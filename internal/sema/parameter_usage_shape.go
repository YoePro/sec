package sema

import "sec/internal/ast"

// walkCompilerKnownPointerAccess consumes Sema's resolved Ptr identity. Address
// formation is attributed to the receiver Place, not a fictitious Ptr field.
// Only a canonical sequence's element pointer requires contiguous sequence
// storage; a scalar or struct address does not acquire sequence shape demand.
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Raw pointer/address formation", "Contiguous-sequence demand"
//   - rules/compiler/compiler_known_members.md — "Ptr on strings", "Ptr on arrays", "Ptr on slices"
func (b *parameterUsageBuilder) walkCompilerKnownPointerAccess(expression ast.Expression) bool {
	member, ok := expression.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return false
	}
	fact, ok := b.analyzer.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]
	if !ok || fact.ID != "CKM-PTR-VALUE" {
		return false
	}
	if !b.markExpression(member.Object, ParameterUseReference, false, ParameterBorrowSufficient, ParameterAddressRequired) {
		return false
	}
	typ := dereferenceType(b.analyzer.expressionTypes[member.Object])
	sequence := typ.Kind == ArrayType || typ.Kind == SliceType || typ.Kind == StringType ||
		(typ.Intrinsic && typ.Name == "list" && len(typ.TypeArgs) == 1)
	if sequence {
		b.addShape(member.Object, ParameterShapeContiguousSequence, 0)
		if parameter, _, ok := b.parameterPlace(member.Object); ok {
			parameter.Demand.Storage = removeParameterStorage(parameter.Demand.Storage, ParameterStorageNone)
			parameter.Demand.Storage = appendUniqueParameterStorage(parameter.Demand.Storage, ParameterStorageContiguous)
		}
	}
	b.walkExpressionChildren(member.Object)
	return true
}

// recordReturnedExactExtent records a fixed-array requirement at a reachable
// return operation, including a borrowed array and fixed arrays materialized
// inside returned carriers. The resolved return value has an exact array type;
// replacing that value by a different extent would change the return contract.
// Merely declaring T[N], indexing, observing Len, or forwarding to a read-only
// callee does not establish this requirement.
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Exact extent and length observation are distinct", "Returning a parameter by value", "Returning a borrow/view"
//   - rules/declarations/functions.md — §3 "Explicit return type", §12 "Return statements"
func (b *parameterUsageBuilder) recordReturnedExactExtent(expression ast.Expression) {
	if parameterUsageNodeIsNil(expression) {
		return
	}
	typ := dereferenceType(b.analyzer.expressionTypes[expression])
	if _, fixed := FixedArrayLength(typ); fixed {
		b.addShape(expression, ParameterShapeExactExtent, 0)
	}
	switch expression := expression.(type) {
	case *ast.PrefixExpression:
		if expression.Operator == "<-" {
			b.recordReturnedExactExtent(expression.Right)
		}
	case *ast.TryExpression:
		b.recordReturnedExactExtent(expression.Expression)
	case *ast.StructLiteral:
		for _, field := range expression.Fields {
			b.recordReturnedExactExtent(field.Value)
		}
	case *ast.ArrayLiteral:
		for _, element := range expression.Elements {
			b.recordReturnedExactExtent(element)
		}
	case *ast.OkExpression:
		b.recordReturnedExactExtent(expression.Value)
		for _, argument := range expression.Arguments {
			b.recordReturnedExactExtent(argument)
		}
	case *ast.ErrExpression:
		b.recordReturnedExactExtent(expression.Value)
		for _, argument := range expression.Arguments {
			b.recordReturnedExactExtent(argument)
		}
	}
}
