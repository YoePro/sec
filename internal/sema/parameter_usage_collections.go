package sema

import "sec/internal/ast"

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
