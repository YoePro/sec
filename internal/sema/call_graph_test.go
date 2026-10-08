package sema

import (
	"testing"

	"sec/internal/lexer"
)

// TestCallGraphExposesExecutionSpecificSCCViews verifies that task and thread
// cycles remain distinct from each other and from same-stack recursion while
// the complete execution view retains both cycles.
//
// Rules:
//   - rules/analysis/call_graph.md — "Spawn cycles"
//   - rules/analysis/call_graph.md — "Thread-start cycles"
//   - rules/analysis/call_graph.md — "Complete execution SCC"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func TestCallGraphExposesExecutionSpecificSCCViews(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
module main

fn TaskA() void {
	let child := spawn task TaskB()
	detach child
}

fn TaskB() void {
	let child := spawn task TaskA()
	detach child
}

fn ThreadA() void {
	let child := spawn thread ThreadB()
	detach child
}

fn ThreadB() void {
	let child := spawn thread ThreadA()
	detach child
}

fn main() void {
	let taskChild := spawn task TaskA()
	let threadChild := spawn thread ThreadA()
	detach taskChild
	detach threadChild
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	taskA := callGraphNodeIDByName(t, graph, "TaskA")
	threadA := callGraphNodeIDByName(t, graph, "ThreadA")

	assertCallGraphComponentNames(t, graph.TaskSpawnSCC(taskA), "TaskA", "TaskB")
	assertCallGraphComponentNames(t, graph.ThreadStartSCC(threadA), "ThreadA", "ThreadB")
	assertCallGraphComponentNames(t, graph.CompleteExecutionSCC(taskA), "TaskA", "TaskB")
	assertCallGraphComponentNames(t, graph.CompleteExecutionSCC(threadA), "ThreadA", "ThreadB")
	if !graph.IsInTaskSpawnCycle(taskA) || !graph.IsInThreadStartCycle(threadA) {
		t.Fatal("mutual task and thread cycles were not classified")
	}

	assertCallGraphComponentNames(t, graph.ThreadStartSCC(taskA), "TaskA")
	assertCallGraphComponentNames(t, graph.TaskSpawnSCC(threadA), "ThreadA")
	if graph.IsInThreadStartCycle(taskA) || graph.IsInTaskSpawnCycle(threadA) {
		t.Fatal("execution cycle leaked into the wrong relationship view")
	}
	if graph.IsSameStackRecursive(taskA) || graph.IsSameStackRecursive(threadA) {
		t.Fatal("execution-boundary cycles must not become same-stack recursion")
	}
}

// TestCallGraphExposesProcessLaunchSCCView verifies the process-specific and
// complete views using semantic graph facts directly. Source-level process
// spawn remains separately unsupported by the frontend.
//
// Rules:
//   - rules/analysis/call_graph.md — "Process-launch cycles"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func TestCallGraphExposesProcessLaunchSCCView(t *testing.T) {
	graph := newCallGraph()
	processA := Function{Name: "ProcessA", Module: "main", Token: lexer.Token{File: "process.sec", Line: 1, Column: 1}}
	processB := Function{Name: "ProcessB", Module: "main", Token: lexer.Token{File: "process.sec", Line: 2, Column: 1}}
	processAID := graph.addCallable(processA)
	processBID := graph.addCallable(processB)
	graph.addCall(processAID, processB, lexer.Token{File: "process.sec", Line: 4, Column: 2}, CallDispatchDirect, CallExecutionSpawnProcess)
	graph.addCall(processBID, processA, lexer.Token{File: "process.sec", Line: 7, Column: 2}, CallDispatchDirect, CallExecutionSpawnProcess)

	assertCallGraphComponentNames(t, graph.ProcessLaunchSCC(processAID), "ProcessA", "ProcessB")
	assertCallGraphComponentNames(t, graph.CompleteExecutionSCC(processAID), "ProcessA", "ProcessB")
	assertCallGraphComponentNames(t, graph.TaskSpawnSCC(processAID), "ProcessA")
	if !graph.IsInProcessLaunchCycle(processAID) {
		t.Fatal("mutual process-launch cycle was not classified")
	}
	if graph.IsSameStackRecursive(processAID) {
		t.Fatal("process-launch cycle must not become same-stack recursion")
	}
}

// TestCallGraphClassifiesSingleNodeSelfSpawn verifies that SCC cardinality one
// does not hide a direct task-spawn self edge.
//
// Rules:
//   - rules/analysis/call_graph.md — "Spawn cycles"
func TestCallGraphClassifiesSingleNodeSelfSpawn(t *testing.T) {
	graph := newCallGraph()
	self := Function{Name: "Self", Module: "main", Token: lexer.Token{File: "self.sec", Line: 1, Column: 1}}
	selfID := graph.addCallable(self)
	graph.addCall(selfID, self, lexer.Token{File: "self.sec", Line: 3, Column: 2}, CallDispatchDirect, CallExecutionSpawnTask)

	assertCallGraphComponentNames(t, graph.TaskSpawnSCC(selfID), "Self")
	if !graph.IsInTaskSpawnCycle(selfID) {
		t.Fatal("direct task self-spawn was not classified as a cycle")
	}
	if graph.IsSameStackRecursive(selfID) {
		t.Fatal("direct task self-spawn must not become same-stack recursion")
	}
}

func assertCallGraphComponentNames(t *testing.T, nodes []CallableNode, want ...string) {
	t.Helper()
	if len(nodes) != len(want) {
		t.Fatalf("component = %+v, want names %v", nodes, want)
	}
	for index, name := range want {
		if nodes[index].Name != name {
			t.Fatalf("component[%d] = %q, want %q (component %+v)", index, nodes[index].Name, name, nodes)
		}
	}
}
