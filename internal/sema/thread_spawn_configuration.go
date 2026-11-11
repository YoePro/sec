package sema

import "sec/internal/ast"

// prepareNamedThreadConfiguration resolves the configuration in the spawning
// execution before the child callable/arguments. Ownership is validated and
// committed only after the complete source spawn has passed validation.
// Rules: rules/concurrency/threads.md — §§10(3–4),20(1–3);
// rules/concurrency/spawn.md — §4(1),(10).
func (a *Analyzer) prepareNamedThreadConfiguration(expr *ast.SpawnExpression) (Function, bool) {
	function := Function{Name: "spawn thread", Parameters: []FunctionParameter{{Name: "config", Type: a.types["ThreadConfig"], Consuming: true}}}
	if expr.Configuration == nil {
		return function, true
	}
	actual, _ := a.inferCallArgumentExpression(expr.Configuration)
	if actual.Kind == InvalidType || !a.checkInitializerType(function.Parameters[0].Type, actual, explicitMoveSource(expr.Configuration)) {
		return function, false
	}
	return function, true
}

// consumeNamedThreadConfiguration reuses ordinary consuming-argument proof and
// transfer publication, including reusable-source markers, partial Places and
// source availability. No target creation/failure transaction is inferred.
// Rules: rules/concurrency/threads.md — §§10(3),20(3–6);
// rules/memory/copy_move.md — §§8.1–8.3;
// rules/declarations/functions.md — §9.
func (a *Analyzer) consumeNamedThreadConfiguration(expr *ast.SpawnExpression, function Function) bool {
	if expr.Configuration == nil {
		return true
	}
	// Child argument preparation may itself have selected this same Place
	// for transfer. Recheck availability without evaluating configuration a
	// second time; ordinary ownership validation expects that primary check.
	if place, ok := a.resolvePlace(explicitMoveSource(expr.Configuration)); ok && a.checkPlaceAvailableForRead(place, expressionToken(expr.Configuration)) {
		return false
	}
	arguments := []ast.Expression{expr.Configuration}
	if !a.validateCallArgumentOwnership(function, arguments, nil, nil) {
		return false
	}
	a.markMovedCallArguments(function, arguments, nil, false)
	return true
}
