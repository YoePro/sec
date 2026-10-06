package sema

import "sec/internal/ast"

// recordFunctionValueCalleeBinding publishes the lexical declaration selected
// by the optimized function-value call lookup. Parameter analysis and tooling
// consume this same identity instead of reconstructing it from a name after
// the callable scope has ended. Type/capability resolution remains unchanged.
// Rules: rules/declarations/lambda-functions.md — §§9,11–12,39 (callable capability, identity and direct-call optimization);
// rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses";
// rules/analysis/closure_analysis.md — "Callable-flow analysis".
func (a *Analyzer) recordFunctionValueCalleeBinding(callee ast.Expression, symbol Symbol) {
	a.expressionTypes[callee] = symbol.Type
	if identifier, ok := callee.(*ast.Identifier); ok {
		a.bindDefinition(identifier.Token, symbol.Token)
	}
}
