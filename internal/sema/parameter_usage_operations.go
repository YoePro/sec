package sema

import (
	"sec/internal/ast"
)

// recordCaptureCreationDemand consumes validated capture transfer facts at
// closure creation, which executes in the enclosing callable. Lambda bodies
// execute in a separate callable and cannot contribute local demand by name.
// Capture/environment dependency demand remains partial and cannot authorize
// positive narrowing advice; separate lambda parameter summaries describe only
// the arguments of an invocation, not captured environment storage.
// Rules: rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses",
// "Unknown critical dimensions block narrowing";
// rules/analysis/closure_analysis.md — "Capture record", "Callable creation".
func (b *parameterUsageBuilder) recordCaptureCreationDemand(lambda *ast.LambdaExpression) {
	captures, ok := b.analyzer.ResolvedLambdaCapturesOf(lambda)
	if !ok {
		return
	}
	for _, capture := range captures {
		id := b.analyzer.bindingIDs[sourceTokenLocation(capture.SourcePlace.RootToken)]
		parameter := b.byBinding[id]
		if parameter == nil {
			continue
		}
		if parameter.Demand.Access == ParameterAccessUnused {
			parameter.Demand.Access = ParameterAccessRead
		}
		kind := ParameterUseRead
		if capture.Transfer == CaptureTransferMove {
			kind = ParameterUseMove
			parameter.Demand.Ownership = strongerOwnership(parameter.Demand.Ownership, ParameterConsumptionRequired)
		}
		parameter.Demand.Precision = strongerPrecision(parameter.Demand.Precision, ParameterDemandPartial)
		parameter.Demand.Shapes = appendUniqueShape(parameter.Demand.Shapes, ParameterShapeWholeValue)
		parameter.Uses = append(parameter.Uses, ParameterUse{Kind: kind, Source: capture.Source, Place: cloneEscapePlace(capture.SourcePlace)})
	}
}

// walkCompilerKnownStructuralCollectionCall records the capability demand of
// compiler-known operations that can change a collection's extent or backing
// relation. It consumes the canonical member-resolution fact produced by Sema
// so an unrelated user method with the same spelling is never classified as a
// structural collection operation.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Mutation demand" / "Structural mutation"
//   - rules/analysis/parameter_usage_analysis.md — "Operation-to-demand transfer" / "Structural collection operations"
func (b *parameterUsageBuilder) walkCompilerKnownStructuralCollectionCall(call *ast.CallExpression) bool {
	memberExpression, ok := call.Callee.(*ast.MemberExpression)
	if !ok || memberExpression == nil || memberExpression.Property == nil {
		return false
	}
	member, ok := b.analyzer.compilerKnownMemberFacts[sourceTokenLocation(memberExpression.Property.Token)]
	if !ok || !member.StructuralMutation {
		return false
	}

	b.markStructuralCollectionMutation(memberExpression.Object)
	b.walkExpressionChildren(memberExpression.Object)
	for _, argument := range call.Arguments {
		b.walkExpression(argument)
	}
	return true
}

// markStructuralCollectionMutation preserves structural mutation as a stronger
// demand than element/field mutation while retaining the canonical receiver
// Place as evidence for interprocedural propagation and tooling.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Structural mutation"
//   - rules/analysis/parameter_usage_analysis.md — "Structural collection operations"
func (b *parameterUsageBuilder) markStructuralCollectionMutation(expression ast.Expression) {
	parameter, place, ok := b.parameterPlace(expression)
	if !ok {
		return
	}
	parameter.Demand.Access = strongerAccess(parameter.Demand.Access, ParameterAccessWrite)
	parameter.Demand.Mutation = strongerMutation(parameter.Demand.Mutation, ParameterStructuralMutation)
	parameter.Uses = append(parameter.Uses, ParameterUse{
		Kind:   ParameterUseCall,
		Source: expressionToken(expression),
		Place:  cloneEscapePlace(place),
	})
}

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
