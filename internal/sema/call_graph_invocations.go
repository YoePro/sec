package sema

import "sec/internal/ast"

// recordResolvedCallGraph publishes the validated invocation's target set and
// execution relationship. Spawned bodies keep their panic effects in the child;
// synchronous preparation calls retain the parent's ordinary effect checks.
// Rules: rules/analysis/call_graph.md — "Call-site record", "Spawn effect transfer",
// "Open callable contract"; rules/errors/panic.md §§6–9,19,21.
func (a *Analyzer) recordResolvedCallGraph(expr *ast.CallExpression, function Function, dispatch CallDispatchKind, receiver Type, isMethodCall bool, contractReceiver Type) {
	execution, recordCall := a.callGraphExecutionForCall(expr)
	if !a.summaryPass && a.callGraphPathReachable && recordCall {
		if dispatch == CallDispatchInterface {
			a.recordInterfaceGraphCall(contractReceiver, function, callCalleeDefinitionToken(expr), execution)
		} else if !sameStackExecution(execution) && isMethodCall && dereferenceType(receiver).Kind == GenericType {
			// A template constraint is an open child target, not the
			// empty abstract method body and not a parent panic source.
			// Rules: rules/analysis/call_graph.md — "Open callable contract";
			// rules/errors/panic.md §§6–9,21(4).
			contract := functionTypeGraphContract(functionTypeFromFunction(function))
			if contract != nil {
				a.callGraph.addTargetSetCall(a.currentCallable, CallableTargetSet{HasOpenContract: true, OpenContract: contract.ID, Contract: contract}, callCalleeDefinitionToken(expr), CallDispatchInterface, execution)
			}
		} else {
			a.callGraph.addCall(a.currentCallable, function, callCalleeDefinitionToken(expr), dispatch, execution)
		}
		if sameStackExecution(execution) {
			a.recordForeignAbortEffect(function, callCalleeDefinitionToken(expr))
		}
		a.recordForeignAllocationEffect(function, callCalleeDefinitionToken(expr))
		a.recordForeignBlockingEffect(function, callCalleeDefinitionToken(expr))
		// rules/errors/panic.md § 21(3)–(4): a method called through an
		// interface reference or a constrained generic parameter has no
		// concrete body here, so its panic behavior is unknown.
		if isMethodCall && dereferenceType(receiver).Kind == GenericType {
			a.recordGenericConstraintCall(receiver, function, callCalleeDefinitionToken(expr), execution)
		}
		if sameStackExecution(execution) && (dispatch == CallDispatchInterface || (isMethodCall && dereferenceType(receiver).Kind == GenericType)) {
			a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicUnknownCallee, Source: callCalleeDefinitionToken(expr)})
			a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{Kind: ArenaEffectUnknownCallee, Source: callCalleeDefinitionToken(expr), UnknownAllocation: true})
			a.recordUnknownBlockingCallee(callCalleeDefinitionToken(expr))
		}
	}
}

// callGraphExecutionForCall classifies only the selected spawn invocation as
// child execution; nested argument/receiver evaluations remain synchronous.
// Rules: rules/concurrency/spawn.md §§3,16; rules/errors/panic.md §§6–9,21.
func (a *Analyzer) callGraphExecutionForCall(expr *ast.CallExpression) (CallExecutionRelation, bool) {
	if expr == a.spawnCallExpression {
		if a.spawnCallExecution == "" {
			return "", false
		}
		return a.spawnCallExecution, true
	}
	return CallExecutionSynchronous, true
}
