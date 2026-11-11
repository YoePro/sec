package sema

import "sec/internal/ast"

// inferCancellationContextCall enforces the exact Context/ContextSource method
// arity and temporal argument types. It does not implement runtime state or
// parent/child lifetime proof; those remain separate compiler capabilities.
// Rules: rules/concurrency/cancellation.md — §§37–38,43;
// rules/types/temporal.md — §5.5; rules/errors/panic.md — §21(4);
// rules/memory/allocation.md — §24 (unknown effects are not freedom proof).
func (a *Analyzer) inferCancellationContextCall(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	value := expressionValue{Display: expr.String()}
	if !a.checkCompilerKnownCallArity(expr, member.Receiver+"."+member.Name, len(member.Parameters), len(member.Parameters)) {
		return Type{Kind: InvalidType}, value, true
	}
	for i, parameter := range member.Parameters {
		expected := a.refinedCompilerKnownType(parameter.Type)
		argument := explicitMoveSource(expr.Arguments[i])
		actual, _ := a.inferExpressionWithExpected(argument, expected)
		a.expressionTypes[expr.Arguments[i]] = actual
		if actual.Kind == InvalidType || !a.checkInitializerType(expected, actual, expr.Arguments[i]) {
			return Type{Kind: InvalidType}, value, true
		}
	}
	function := Function{Name: member.Receiver + "." + member.Name, Parameters: member.Parameters, ReturnType: member.Result}
	if !a.validateCallArgumentOwnership(function, expr.Arguments, nil, expr) {
		return Type{Kind: InvalidType}, value, true
	}
	a.markMovedCallArguments(function, expr.Arguments, nil, false)
	// Signatures do not prove runtime effect freedom. Until the runtime
	// contracts are represented, retain the ordinary unknown-callee boundary.
	if !a.summaryPass && a.callGraphPathReachable {
		source := callCalleeDefinitionToken(expr)
		a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicUnknownCallee, Source: source})
		a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{Kind: ArenaEffectUnknownCallee, Source: source, UnknownAllocation: true})
		a.recordUnknownBlockingCallee(source)
	}
	return a.refinedCompilerKnownType(member.Result), value, true
}
