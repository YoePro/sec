package sema

import "sec/internal/ast"

// sameStackExecution identifies the currently represented relations that keep
// cleanup and ordinary invocation in the caller's execution context.
// Rules: rules/analysis/call_graph.md — "Same-stack execution".
func sameStackExecution(execution CallExecutionRelation) bool {
	return execution == CallExecutionSynchronous || execution == CallExecutionDeferred
}

// recordDeferCallable gives every validated defer body a distinct synthetic
// node, including bodies without calls. Only reachable registration contributes
// a cleanup edge; body checking keeps the original flow reachability. Identity
// is source-local, scoped by its enclosing callable, rather than AST addresses
// or traversal counters. Repeated loop registrations share this may-graph node.
// Rules: rules/analysis/call_graph.md — "`defer` bodies", "Callable node identity",
// "Dispatch kinds", "Same-stack execution";
// rules/control-flow/defer.md — §§2–3, 12–13, 29.
func (a *Analyzer) recordDeferCallable(stmt *ast.DeferStatement) CallableID {
	if a.summaryPass || a.currentCallable == "" || a.callGraph == nil {
		return a.currentCallable
	}
	caller := a.currentCallable
	body := CallableBodyID("callable-body|" + a.callableIdentityKey("defer", stmt.Token))
	id := CallableID(body)
	if _, exists := a.callGraph.nodes[id]; !exists {
		a.callGraph.nodes[id] = CallableNode{
			ID: id, Kind: CallableBodyDefer, Name: "defer", Module: a.currentModule,
			Declaration: stmt.Token,
		}
		a.callGraph.nodeOrder = append(a.callGraph.nodeOrder, id)
		a.callGraph.bodyNodes[body] = id
	}
	if a.callGraphPathReachable {
		a.callGraph.addTargetSetCall(caller, exactCallableTargetSet(body), stmt.Token, CallDispatchGenerated, CallExecutionDeferred)
	}
	return id
}
