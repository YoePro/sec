package main

import (
	"strings"
	"testing"
)

// TestHoverShowsExecutionBoundaryCycles verifies that the LSP consumes the
// compiler-owned task and thread SCC classifications without reporting either
// cycle as same-stack recursion.
//
// Rules:
//   - rules/analysis/call_graph.md — "LSP behavior"
//   - rules/analysis/call_graph.md — "Spawn cycles"
//   - rules/analysis/call_graph.md — "Thread-start cycles"
func TestHoverShowsExecutionBoundaryCycles(t *testing.T) {
	source := `module main

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
`
	uri := "file:///tmp/sec-lsp-call-graph-cycle-hover/main.sec"
	taskDeclaration := strings.Index(source, "TaskA()")
	taskHover, ok := hoverForSource(uri, source, offsetPosition(source, taskDeclaration))
	if !ok || !strings.Contains(taskHover.Contents.Value, "Task-spawn cycle: `TaskA`, `TaskB`") {
		t.Fatalf("task-cycle hover = %+v, %v", taskHover, ok)
	}
	if strings.Contains(taskHover.Contents.Value, "Same-stack recursion") || strings.Contains(taskHover.Contents.Value, "Thread-start cycle") {
		t.Fatalf("task-cycle hover conflates execution views: %s", taskHover.Contents.Value)
	}

	threadDeclaration := strings.Index(source, "ThreadA()")
	threadHover, ok := hoverForSource(uri, source, offsetPosition(source, threadDeclaration))
	if !ok || !strings.Contains(threadHover.Contents.Value, "Thread-start cycle: `ThreadA`, `ThreadB`") {
		t.Fatalf("thread-cycle hover = %+v, %v", threadHover, ok)
	}
	if strings.Contains(threadHover.Contents.Value, "Same-stack recursion") || strings.Contains(threadHover.Contents.Value, "Task-spawn cycle") {
		t.Fatalf("thread-cycle hover conflates execution views: %s", threadHover.Contents.Value)
	}
}
