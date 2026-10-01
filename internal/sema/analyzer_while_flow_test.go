package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Folded while conditions publish one immutable compiler fact instead of
// forcing return analysis or tooling to reconstruct constant expressions.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§19–20 "Constant conditions" and "Non-continuing while true"
//   - rules/control-flow/flowcontrol_while.md — §28 "Sema and flow-analysis requirements"
func TestResolvedWhileFlowRetainsFoldedConditionAndExitFacts(t *testing.T) {
	input := `
module main

fn Always() void {
	while 1 + 1 == 2 {}
}

fn Never() void {
	while 3 < 2 {}
}

fn Runtime(flag: bool) void {
	while flag {}
}
`
	parsed := parser.New(lexer.New(input))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parsed.Errors())
	}
	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(program)
	assertSemaErrors(t, errors, nil)

	loops := []*ast.WhileStatement{}
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Body == nil || len(function.Body.Statements) == 0 {
			continue
		}
		loop, ok := function.Body.Statements[0].(*ast.WhileStatement)
		if ok {
			loops = append(loops, loop)
		}
	}
	if len(loops) != 3 {
		t.Fatalf("while count = %d, want 3", len(loops))
	}

	always, ok := analyzer.ResolvedWhileFlowOf(loops[0])
	if !ok || !always.ConditionKnown || !always.ConditionValue || always.HasReachableBreak || always.ContinuesAfterLoop {
		t.Fatalf("folded true while flow = %+v, found=%v", always, ok)
	}
	never, ok := analyzer.ResolvedWhileFlowOf(loops[1])
	if !ok || !never.ConditionKnown || never.ConditionValue || never.HasReachableBreak || !never.ContinuesAfterLoop {
		t.Fatalf("folded false while flow = %+v, found=%v", never, ok)
	}
	runtime, ok := analyzer.ResolvedWhileFlowOf(loops[2])
	if !ok || runtime.ConditionKnown || runtime.HasReachableBreak || !runtime.ContinuesAfterLoop {
		t.Fatalf("runtime while flow = %+v, found=%v", runtime, ok)
	}

	before := len(analyzer.resolvedWhileFlows)
	if _, found := analyzer.ResolvedWhileFlowOf(&ast.WhileStatement{}); found || len(analyzer.resolvedWhileFlows) != before {
		t.Fatal("unknown while lookup mutated resolved facts")
	}
}

// A folded-true while without a reachable current-loop break is
// non-continuing. A reachable break restores continuation, while a break in a
// folded-false branch does not.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§19–20
//   - rules/tooling/diagnostics.md — §21 "Proven unreachable and dead code"
func TestFoldedTrueWhileControlsReturnAndReachability(t *testing.T) {
	if errors := analyzeSourceRaw(t, `
module main
fn Infinite() int {
	while 1 + 1 == 2 {}
}

fn Branch(flag: bool) int {
	if flag {
		while 2 * 2 == 4 {}
	} else {
		return 1
	}
}
`); len(errors) != 0 {
		t.Fatalf("folded infinite loop errors: %v", errors)
	}

	errors := analyzeSource(t, `
fn Unreachable() void {
	while 3 > 2 {}
	discard 1
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.UnreachableStatement {
		t.Fatalf("post-loop diagnostics = %+v, want one S3001", errors)
	}

	errors = analyzeSourceRaw(t, `
module main
fn Breaks(stop: bool) int {
	while 4 == 4 {
		if stop {
			break
		}
	}
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "must return int") {
		t.Fatalf("reachable-break diagnostics = %+v, want missing return", errors)
	}

	errors = analyzeSource(t, `
fn IgnoresImpossibleBreak() int {
	while 5 == 5 {
		if 1 > 2 {
			break
		}
	}
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.UnreachableStatement || strings.Contains(errors[0].Message, "must return") {
		t.Fatalf("impossible-break diagnostics = %+v, want only S3001", errors)
	}
}

// A folded-false body remains semantically checked but cannot contribute call
// edges or panic effects to its enclosing callable.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
//   - rules/analysis/call_graph.md — reachable call edges
//   - rules/analysis/effect_analysis.md — reachable effect provenance
func TestFoldedFalseWhileBodyDoesNotContributeReachableCallsOrEffects(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Dead() void {}

@noPanic
fn Never() void {
	while 2 < 1 {
		Dead()
		panic "dead"
	}
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.UnreachableStatement {
		t.Fatalf("diagnostics = %+v, want only folded-false S3001", errors)
	}

	graph := analyzer.CallGraph()
	neverID := callGraphNodeIDByName(t, graph, "Never")
	if outgoing := graph.Outgoing(neverID); len(outgoing) != 0 {
		t.Fatalf("Never outgoing calls = %+v, want no dead-body edge", outgoing)
	}
	if summary := graph.EffectSummary(neverID); summary.MayPanic || len(summary.DirectEffects) != 0 {
		t.Fatalf("Never effects = %+v, want no dead-body panic effect", summary)
	}
}

// A folded-false loop has only its zero-iteration exit. Body analysis must
// retain diagnostics without committing ownership or Arena generation changes
// to the state following the loop.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
//   - rules/control-flow/flowcontrol_while.md — §§28–29 zero-iteration flow and CFG requirements
//   - rules/memory/ownership.md — moves occur only on reachable control-flow paths
//   - rules/memory/arena.md — Arena generations follow reachable reset and release operations
func TestFoldedFalseWhileBodyDoesNotCommitRuntimeState(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Ownership() void {
	let mut pair := Pair { first: Session { value: 1 } }
	while 2 < 1 {
		let first :<- pair.first
		discard first
	}
	let first :<- pair.first
	discard first
}

fn ArenaState() Result[void, AllocationError] {
	let mut arena: Arena := Arena {}
	let storage := try arena.Alloc[byte](16u)
	while 3 == 4 {
		arena.Reset()
	}
	let length := storage.len
	discard length
	return Ok()
}
`)
	if len(errors) != 2 {
		t.Fatalf("diagnostics = %+v, want only two folded-false S3001 diagnostics", errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.UnreachableStatement {
			t.Fatalf("diagnostic = %+v, want only S3001", err)
		}
	}
}
