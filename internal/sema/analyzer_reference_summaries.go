package sema

import "sec/internal/ast"

// inferFunctionReferenceSummaries solves recursive reference-return origins in
// bounded deterministic rounds, publishing Unknown if refinement is exhausted.
// Rules: rules/compiler/compiler_analysis.md — §14(1–3), §15(3–5);
// rules/memory/lifetime_analysis.md — §28 "Separate compilation and summaries", §28.3 "Recursive calls".
func (a *Analyzer) inferFunctionReferenceSummaries(program *ast.Program) analysisStepResult {
	functionCount := 0
	for _, overloads := range a.functions {
		functionCount += len(overloads)
	}
	if functionCount == 0 {
		return analysisStepResult{Converged: true}
	}
	a.summaryPass = true
	defer func() { a.summaryPass = false }()
	iterationLimit := functionCount + 1
	if configured := a.analysisBudget.MaxSummaryIterations; configured > 0 && configured < iterationLimit {
		iterationLimit = configured
	}
	return runAnalysisFixedPoint(iterationLimit, func() bool {
		before := copyFunctionReferenceSummaries(a.functions)
		a.analyzeFunctionBodies(program)
		a.analyzeImplBodies(program)
		return !functionReferenceSummariesEqual(before, a.functions)
	}, a.widenFunctionReferenceSummaries)
}

// widenFunctionReferenceSummaries preserves soundness when an interactive
// resource limit is reached: unresolved reference-returning calls become
// unknown rather than retaining a potentially incomplete proof.
// Rules: rules/compiler/compiler_analysis.md — §14(2–3), §15(3–5).
func (a *Analyzer) widenFunctionReferenceSummaries() {
	for name, overloads := range a.functions {
		for index := range overloads {
			if !typeContainsReference(overloads[index].ReturnType, map[string]bool{}) {
				continue
			}
			overloads[index].HasReturnOrigin = true
			overloads[index].ReturnOrigin = localReferenceOrigin{Unknown: true, Ambiguous: true}
		}
		a.functions[name] = overloads
	}
}

// copyFunctionReferenceSummaries detaches return origins for round comparison.
// Rules: rules/compiler/compiler_analysis.md — §14(2); rules/memory/lifetime_analysis.md — §28 "Separate compilation and summaries", §28.3 "Recursive calls".
func copyFunctionReferenceSummaries(functions map[string][]Function) map[string][]Function {
	out := make(map[string][]Function, len(functions))
	for name, overloads := range functions {
		out[name] = make([]Function, len(overloads))
		for index, function := range overloads {
			function.ReturnOrigin = cloneLocalReferenceOrigin(function.ReturnOrigin)
			out[name][index] = function
		}
	}
	return out
}

// functionReferenceSummariesEqual compares semantic origin facts, not pass progress.
// Rules: rules/compiler/compiler_analysis.md — §14(2); rules/memory/lifetime_analysis.md — §28 "Separate compilation and summaries", §28.3 "Recursive calls".
func functionReferenceSummariesEqual(left, right map[string][]Function) bool {
	if len(left) != len(right) {
		return false
	}
	for name, leftOverloads := range left {
		rightOverloads, ok := right[name]
		if !ok || len(leftOverloads) != len(rightOverloads) {
			return false
		}
		for index, leftFunction := range leftOverloads {
			rightFunction := rightOverloads[index]
			if leftFunction.HasReturnOrigin != rightFunction.HasReturnOrigin {
				return false
			}
			if leftFunction.HasReturnOrigin && !sameReferenceOrigin(leftFunction.ReturnOrigin, rightFunction.ReturnOrigin) {
				return false
			}
		}
	}
	return true
}
