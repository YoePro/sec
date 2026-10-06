package sema

import (
	"encoding/json"
	"fmt"
	"sort"

	"sec/internal/ast"
)

// testCallableID encodes structured semantic identity without conflating path
// components, display labels, source positions, or generated linker symbols.
// The graph scope supplies CompilationPlan compatibility independently.
// Rules: rules/tooling/testing.md — §§6.1, 6.4, 33.1;
// rules/analysis/call_graph.md — "Callable node identity", "Test roots".
func testCallableID(identity TestIdentity) CallableID {
	encoded, _ := json.Marshal(identity) // This string/slice-only type cannot fail.
	return CallableID("test-body|" + string(encoded))
}

// recordTestCallable retains a validated test body's internal executable node
// without introducing a Function, function value, or source-visible name.
// Root membership is selected separately by CallGraphForTestPlan.
// Rules: rules/tooling/testing.md — §§5.7, 9.1, 33.1;
// rules/analysis/call_graph.md — "Callable node", "Test roots".
func (a *Analyzer) recordTestCallable(declaration *ast.TestDeclaration) CallableID {
	metadata, valid := a.resolvedTestMetadata[declaration]
	if !valid || a.summaryPass || a.callGraph == nil {
		return ""
	}
	id := testCallableID(metadata.Identity)
	if _, exists := a.callGraph.nodes[id]; !exists {
		a.callGraph.nodes[id] = CallableNode{
			ID: id, Kind: CallableBodyTest, Name: metadata.Name,
			Module: metadata.Identity.Module, Declaration: declaration.Token,
		}
		a.callGraph.nodeOrder = append(a.callGraph.nodeOrder, id)
		a.callGraph.bodyNodes[CallableBodyID(id)] = id
	}
	return id
}

// CallGraphForTestPlan creates a detached canonical graph view whose entry
// roots are exactly the explicit selected tests in the supplied plan scope.
// It removes production entry roots without deleting shared callable facts;
// ordinary reachability determines which helpers, cleanup and worker entries
// execute. Empty selection means no roots, never implicit "select all".
// Invalid/unknown/duplicate selections and failed Sema snapshots are rejected
// transactionally. This API owns graph selection, not source discovery, harness
// generation, linking or runtime execution in the compiler driver.
// Rules: rules/analysis/call_graph.md — "Test roots", "Root reachability classes",
// "One graph per `CompilationPlan`"; rules/tooling/testing.md — §§5.7, 26, 27, 34.4.
func (a *Analyzer) CallGraphForTestPlan(scope CallGraphScope, selected []TestIdentity) (*CallGraph, error) {
	if a == nil || a.callGraph == nil || len(a.errors) != 0 {
		return nil, fmt.Errorf("test graph requires successful semantic analysis")
	}
	if scope.Module == "" || scope.CompilationPlan == "" || scope.CompilerModel == "" {
		return nil, fmt.Errorf("test graph requires complete module, compilation-plan and compiler-model scope")
	}
	ids := make([]CallableID, 0, len(selected))
	seen := map[CallableID]bool{}
	for _, identity := range selected {
		id := testCallableID(identity)
		node, exists := a.callGraph.nodes[id]
		if !exists || node.Kind != CallableBodyTest {
			return nil, fmt.Errorf("selected test identity is not a validated test: %s", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate selected test identity: %s", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	graph := a.callGraph.clone()
	graph.roots = map[CallRootID]CallRoot{}
	graph.rootOrder = nil
	for _, id := range ids {
		node := graph.nodes[id]
		rootID := graph.addRoot(CallRootTestEntry, id, node.Declaration)
		root := graph.roots[rootID]
		delete(graph.roots, rootID)
		encodedScope, _ := json.Marshal(scope)
		root.ID = CallRootID(string(rootID) + "|" + string(encodedScope))
		root.Scope = scope
		graph.roots[root.ID] = root
		graph.rootOrder[len(graph.rootOrder)-1] = root.ID
	}
	return graph.ForCompilationPlan(scope)
}

// validTestRootScope prevents publishing or restoring selected test roots under
// another plan/model, and rejects test entries targeting ordinary functions.
// Rules: rules/analysis/call_graph.md — "Test roots", "One graph per `CompilationPlan`";
// rules/compiler/incremental_compilation.md — §§31–32.
func validTestRootScope(graph *CallGraph, scope CallGraphScope) bool {
	for _, root := range graph.roots {
		if root.Kind == CallRootTestEntry {
			node, exists := graph.nodes[root.Node]
			if !exists || node.Kind != CallableBodyTest || root.Scope != scope {
				return false
			}
		}
	}
	return true
}
