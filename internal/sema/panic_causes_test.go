package sema

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// panicCauseAnalyzer preserves real source identity in canonical fixture facts.
// Rules: rules/errors/panic.md — §§25(3),28(2)–(3).
func panicCauseAnalyzer(t *testing.T, name string) (*Analyzer, *ast.Program, []Error) {
	t.Helper()
	file := "../../testdata/sema/" + name + ".sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if errors := p.Errors(); len(errors) != 0 {
		t.Fatal(errors)
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	return a, program, errors
}

// TestPanicCausesCheckpoint preserves reason/message and call-site provenance
// through workspace persistence; pre-metadata checkpoints cannot authorize reuse.
// Rules: rules/errors/panic.md — §25(3);
// rules/compiler/incremental_compilation.md — §§31–32.
func TestPanicCausesCheckpoint(t *testing.T) {
	a, _, errors := panicCauseAnalyzer(t, "panic_causes_valid")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	g := a.CallGraph()
	scope := CallGraphScope{"main", "linux-amd64", "panic-causes-v1"}
	deps := []CallGraphDependency{{"source", "../../testdata/sema/panic_causes_valid.sec", "source-v1"}}
	var index WorkspaceCallGraphIndex
	lease, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(lease, g); err != nil || !ok {
		t.Fatal(ok, err)
	}
	before, ok := index.Snapshot(scope, deps)
	if !ok {
		t.Fatal("missing published graph")
	}
	checkpoint, err := index.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	current := map[CallGraphScope][]CallGraphDependency{scope: deps}
	restored, err := RestoreWorkspaceCallGraphIndex(checkpoint, current)
	if err != nil {
		t.Fatal(err)
	}
	after, ok := restored.Snapshot(scope, deps)
	if !ok || !reflect.DeepEqual(before.EffectSummary(callGraphNodeIDByName(t, before, "Root")), after.EffectSummary(callGraphNodeIDByName(t, after, "Root"))) {
		t.Fatal("panic metadata lost after restore")
	}
	var envelope graphCheckpoint
	if err := json.Unmarshal(checkpoint, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Version = 3
	old, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreWorkspaceCallGraphIndex(old, current); err == nil {
		t.Fatal("checkpoint without panic message metadata accepted")
	}
}

// TestPanicCausesRetainEverySource verifies multiple leaves, recursive
// convergence, exact call-site witnesses, static messages, reason sets, and
// detached snapshots. A handled arithmetic failure and a proven assertion disappear.
// Rules: rules/errors/panic.md — §§13,15.8,17,25(3)–(4);
// rules/analysis/effect_analysis.md — "Effect causes", "Fixed-point analysis".
func TestPanicCausesRetainEverySource(t *testing.T) {
	a, _, errors := panicCauseAnalyzer(t, "panic_causes_valid")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	g := a.CallGraph()
	id := func(name string) CallableID { return callGraphNodeIDByName(t, g, name) }
	summary := g.EffectSummary(id("Root"))
	if !summary.MayPanic || len(summary.PanicCauses) != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	byName := map[string]EffectDomainCause{}
	for _, cause := range summary.PanicCauses {
		node, _ := g.Node(cause.Callable)
		byName[node.Name] = cause
		if len(cause.Path) != 2 || cause.Path[0].Caller != id("Root") || cause.Path[1].Caller != id("Middle") || cause.Source.File == "" || cause.Source != cause.Effect.Source {
			t.Fatalf("missing exact provenance: %+v", cause)
		}
		for _, site := range cause.Path {
			if site.Source.File != cause.Source.File || site.Source.Line == 0 || site.Source.Column == 0 || !sameStackExecution(site.Execution) || len(site.Targets) != 1 {
				t.Fatal(site)
			}
		}
	}
	if got := byName["Check"].Effect; got == nil || got.Kind != EffectMayPanicAssertion || !got.HasMessage || got.Message != "value must hold" || !reflect.DeepEqual(got.PanicReasonIDs, []diagnostics.PanicReasonID{diagnostics.PanicReasonAssertionFailure}) {
		t.Fatal(got)
	}
	if got := byName["Divide"].Effect; got == nil || got.HasMessage || !reflect.DeepEqual(got.PanicReasonIDs, []diagnostics.PanicReasonID{diagnostics.PanicReasonDivisionByZero, diagnostics.PanicReasonArithmeticOverflow}) {
		t.Fatal(got)
	}
	if got := byName["Stop"].Effect; got == nil || !got.HasMessage || got.Message != "stop here" {
		t.Fatal(got)
	}
	for _, name := range []string{"Protected", "Proven"} {
		if got := g.EffectSummary(id(name)); got.MayPanic || len(got.PanicCauses) != 0 {
			t.Fatal(name, got)
		}
	}
	if got := g.EffectSummary(id("RecursiveB")); !got.MayPanic || len(got.PanicCauses) != 1 || len(got.PanicCauses[0].Path) != 2 {
		t.Fatal(got)
	}
	baseline := g.EffectSummary(id("Root"))
	for i, j := 0, len(g.sites)-1; i < j; i, j = i+1, j-1 {
		g.sites[i], g.sites[j] = g.sites[j], g.sites[i]
	}
	if !reflect.DeepEqual(g.EffectSummary(id("Root")), baseline) {
		t.Fatal("cause order depends on site insertion order")
	}
	summary.PanicCauses[0].Effect.Kind = EffectMayAllocate
	summary.PanicCauses[0].Effect.PanicReasonIDs[0] = diagnostics.PanicReasonForeignAbort
	summary.PanicCauses[0].Path[0].Targets[0] = "changed"
	summary.PanicCauses[0].Path[0].TargetSet.KnownTargets = nil
	if fresh := g.EffectSummary(id("Root")); !reflect.DeepEqual(fresh, baseline) {
		t.Fatal("published cause mutated graph")
	}
	domain := g.DomainSummary(id("Check"))
	for _, cause := range domain.DirectCauses {
		if cause.Effect != nil {
			cause.Effect.PanicReasonIDs[0] = diagnostics.PanicReasonForeignAbort
		}
	}
	for _, cause := range domain.Causes {
		if cause.Effect != nil && cause.Effect.PanicReasonIDs[0] != diagnostics.PanicReasonAssertionFailure {
			t.Fatal("direct/transitive cause alias")
		}
	}
}

// TestPanicCausesAcrossFiles distinguishes the introducing source from the
// caller occurrence and preserves both in the detached cause witness.
// Rules: rules/errors/panic.md — §§25(3),28(2)–(3).
func TestPanicCausesAcrossFiles(t *testing.T) {
	program := &ast.Program{}
	for _, name := range []string{"root", "leaf"} {
		file := "../../testdata/sema/panic_causes/" + name + ".sec"
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		part := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, part.Statements...)
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	g := a.CallGraph()
	causes := g.EffectSummary(callGraphNodeIDByName(t, g, "Root")).PanicCauses
	if len(causes) != 1 || !strings.HasSuffix(causes[0].Source.File, "/leaf.sec") || len(causes[0].Path) != 1 || !strings.HasSuffix(causes[0].Path[0].Source.File, "/root.sec") || causes[0].Effect.Message != "separate source" {
		t.Fatal(causes)
	}
}

// TestNoPanicSelectsPanicCause ignores an earlier unrelated effect instead of
// falsely naming it as the panic source. The related location is the assertion.
// Rules: rules/errors/panic.md — §§21,28(2)–(3).
func TestNoPanicSelectsPanicCause(t *testing.T) {
	a, program, errors := panicCauseAnalyzer(t, "panic_cause_diagnostic_invalid")
	if len(errors) != 1 || errors[0].ID != diagnostics.NoPanicViolation {
		t.Fatal(errors)
	}
	id := callGraphNodeIDByName(t, a.callGraph, "Leaf")
	panicSite := a.callGraph.effects[id][0]
	a.callGraph.effects[id] = append([]EffectSite{{Kind: EffectVolatileRead, Source: lexer.Token{File: "unrelated.sec", Line: 1, Column: 1}}}, a.callGraph.effects[id]...)
	a.errors = nil
	a.errorKeys = nil
	a.validateNoPanicGuarantees(program)
	if len(a.errors) != 1 || !strings.Contains(a.errors[0].Message, string(EffectMayPanicAssertion)) || a.errors[0].PreviousLine != panicSite.Source.Line || a.errors[0].PreviousFile != panicSite.Source.File {
		t.Fatal(a.errors)
	}
}
