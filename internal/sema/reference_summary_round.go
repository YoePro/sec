package sema

import "sec/internal/ast"

// referenceSummaryRoundInputs freezes only source-defined callable inputs for
// one synchronous round. Bodyless/foreign declarations are not pending local
// equations and must never supply bottom instead of Unknown.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3;
// rules/memory/references.md — §9(10), §29(2).
func (a *Analyzer) referenceSummaryRoundInputs(program *ast.Program, functions map[string][]Function) map[sourceTokenKey]Function {
	bodies := map[sourceTokenKey]bool{}
	var collect func(ast.Statement)
	collect = func(statement ast.Statement) {
		if parameterUsageNodeIsNil(statement) {
			return
		}
		switch declaration := statement.(type) {
		case *ast.FunctionDeclaration:
			if declaration.Body != nil && declaration.Name != nil {
				bodies[sourceTokenLocation(declaration.Name.Token)] = true
			}
		case *ast.ImplStatement:
			for _, member := range declaration.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok {
					collect(fn)
				}
			}
		}
	}
	for _, statement := range program.Statements {
		collect(statement)
	}
	inputs := map[sourceTokenKey]Function{}
	for _, overloads := range functions {
		for _, function := range overloads {
			key := sourceTokenLocation(function.Token)
			if !function.Extern && bodies[key] {
				inputs[key] = function
			}
		}
	}
	return inputs
}

// prepareReferenceSummaryCall reads the previous round, never a partially
// updated callee. A local equation without a returned origin is bottom only
// inside inference. It contributes no return alternative until a base case
// arrives; it is never exported as a positive proof to ordinary body checking.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3;
// rules/compiler/compiler_analysis.md — §14(1–3), §15(3–5).
func (a *Analyzer) prepareReferenceSummaryCall(call *ast.CallExpression, function *Function) bool {
	if !a.summaryPass {
		return false
	}
	delete(a.referenceSummaryPending, call)
	input, local := a.referenceSummaryRound[sourceTokenLocation(function.Token)]
	if !local {
		return false
	}
	function.HasReturnOrigin, function.ReturnOrigin = input.HasReturnOrigin, input.ReturnOrigin
	if input.HasReturnOrigin || !typeContainsReference(function.ReturnType, map[string]bool{}) {
		return false
	}
	delete(a.expressionReferenceOrigins, call)
	a.referenceSummaryPending[call] = true
	return true
}

// finishPendingReferenceSummaries replaces unseeded recursive equations with
// explicit Unknown before any consuming analysis runs. No returning-path fact
// was established for them, so an empty equation cannot certify a lifetime.
// Rules: rules/memory/lifetime_analysis.md — §28.3;
// rules/memory/references.md — §9(10).
func (a *Analyzer) finishPendingReferenceSummaries() {
	for name, overloads := range a.functions {
		for index := range overloads {
			function := &overloads[index]
			if _, local := a.referenceSummaryRound[sourceTokenLocation(function.Token)]; local && !function.HasReturnOrigin && typeContainsReference(function.ReturnType, map[string]bool{}) {
				function.HasReturnOrigin = true
				function.ReturnOrigin = localReferenceOrigin{Unknown: true, Ambiguous: true}
			}
		}
		a.functions[name] = overloads
	}
}

// recordFunctionReturnOrigin joins returned origins, leaving unresolved local
// equations at bottom only while inferring summaries. Actual body checking
// receives finalized summaries and records ordinary escape evidence.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3;
// rules/analysis/escape_analysis.md — "Returned references and views", "Return summaries".
func (a *Analyzer) recordFunctionReturnOrigin(returnType Type, expr ast.Expression) {
	if call, ok := expr.(*ast.CallExpression); ok && a.summaryPass && a.referenceSummaryPending[call] {
		return
	}
	if expr == nil {
		return
	}
	if !typeContainsReference(returnType, map[string]bool{}) {
		raw, symbolic := a.ownedReturnEscapeOrigins(expr)
		a.recordReturnEscapeFact(returnType, expr, raw, symbolic)
		return
	}
	candidate := localReferenceOrigin{}
	if typeCarriesReferenceOrigin(returnType) {
		origin, ok := a.directReferenceOrigin(expr)
		if !ok {
			candidate.Unknown = true
			candidate.Ambiguous = true
		} else {
			candidate = origin
		}
	} else {
		candidate.Contained = a.containedReferenceOrigins(expr)
		if len(candidate.Contained) == 0 {
			candidate.Unknown = true
			candidate.Ambiguous = true
		} else {
			recomputeContainedOriginSummary(&candidate)
		}
	}
	rawCandidate := cloneLocalReferenceOrigin(candidate)
	candidate = a.symbolizeFunctionReturnOrigin(candidate)
	a.recordReturnEscapeFact(returnType, expr, rawCandidate, candidate)
	if !a.hasCurrentFunctionSummary {
		a.currentFunctionSummary = candidate
		a.hasCurrentFunctionSummary = true
		return
	}
	a.currentFunctionSummary = mergeLocalReferenceOrigins([]localReferenceOrigin{a.currentFunctionSummary, candidate}, false)
}

// setCallReferenceOrigin instantiates a resolved returned-origin relation at
// its call site. Recursive inference reads an isolated previous-round snapshot.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3;
// rules/memory/references.md — §9(10).
func (a *Analyzer) setCallReferenceOrigin(expr *ast.CallExpression, function Function, args []ast.Expression, isMethodCall bool) {
	if a.prepareReferenceSummaryCall(expr, &function) {
		return
	}
	delete(a.expressionReferenceOrigins, expr)
	if !function.HasReturnOrigin {
		return
	}
	var receiver ast.Expression
	if isMethodCall {
		if member, ok := expr.Callee.(*ast.MemberExpression); ok {
			receiver = member.Object
		}
	}
	origin := a.instantiateFunctionReturnOrigin(function.ReturnOrigin, args, receiver)
	a.expressionReferenceOrigins[expr] = origin
}
