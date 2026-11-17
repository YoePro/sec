package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Fence events preserve source order and duplicates alongside memory accesses,
// without manufacturing volatile access, I/O, allocation, blocking or panic.
// Rules: rules/concurrency/atomics.md — §§26,40,46;
// rules/analysis/effect_analysis.md — §§14–15.
func TestAtomicFenceOrderingEffects(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomic_fence_effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errors, nil)
	graph := a.CallGraph()
	id := callGraphNodeIDByName(t, graph, "Orders")
	effects := graph.EffectSummary(id).DirectEffects
	want := []string{"Acquire", "Release", "AcqRel", "SeqCst", "SeqCst"}
	if len(effects) != len(want) {
		t.Fatalf("fence effects = %+v", effects)
	}
	for index, effect := range effects {
		if effect.Kind != EffectAtomicFence || effect.AtomicOrder != want[index] || effect.Source.Line != index+7 || effect.Source.Column == 0 {
			t.Fatalf("fence order/source lost: %+v", effect)
		}
	}
	effects[0].AtomicOrder = "changed"
	if graph.EffectSummary(id).DirectEffects[0].AtomicOrder != "Acquire" {
		t.Fatal("query aliases stored ordering facts")
	}
	for _, name := range []string{"Orders", "Caller"} {
		id := callGraphNodeIDByName(t, graph, name)
		summary := graph.DomainSummary(id)
		if len(summary.Effects) != 0 || len(summary.UnknownEffects) != 0 || graph.EffectSummary(id).MayPanic {
			t.Fatalf("fence invented may-effects in %s: %+v", name, summary)
		}
	}
	mixed := graph.EffectSummary(callGraphNodeIDByName(t, graph, "Mixed")).DirectEffects
	var kinds []EffectKind
	for _, effect := range mixed {
		kinds = append(kinds, effect.Kind)
	}
	if !reflect.DeepEqual(kinds, []EffectKind{EffectMemoryWrite, EffectAtomicFence, EffectMemoryRead}) || mixed[1].AtomicOrder != "Release" {
		t.Fatalf("memory/fence sequence lost: %+v", mixed)
	}
}

// Invalid orders and source-level lookalikes cannot acquire fence authority.
// Rules: rules/concurrency/atomics.md — §§26,40,45(9).
func TestAtomicFenceEffectsRequireValidatedIdentity(t *testing.T) {
	for _, name := range []string{"atomic_fence_invalid.sec", "atomic_fence_shadowed_valid.sec", "atomic_fence_effects_unreachable_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + name)
		if err != nil {
			t.Fatal(err)
		}
		a, issues := analyzeSourceWithAnalyzerRaw(t, string(data))
		if (len(issues) == 0) != (name == "atomic_fence_shadowed_valid.sec") {
			t.Fatal(issues)
		}
		for _, node := range a.CallGraph().Nodes() {
			for _, effect := range a.CallGraph().EffectSummary(node.ID).DirectEffects {
				if effect.Kind == EffectAtomicFence {
					t.Fatalf("%s published an unauthorized fence: %+v", name, effect)
				}
			}
		}
	}
}

// Workspace snapshots preserve the same ordered fence facts and refuse an
// invalid or missing order instead of reusing incomplete ordering evidence.
// Rules: rules/concurrency/atomics.md — §§26,40;
// rules/compiler/incremental_compilation.md — §§31–33.
func TestAtomicFenceEffectsCheckpoint(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomic_fence_effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	const file = "fence_effects.sec"
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	assertSemaErrors(t, a.Analyze(program), nil)
	graph := a.CallGraph()
	scope := CallGraphScope{"main", "test-plan", "fence-model"}
	deps := []CallGraphDependency{{"source", file, "source-v1"}}
	var index WorkspaceCallGraphIndex
	lease, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(lease, graph); !ok || err != nil {
		t.Fatal(ok, err)
	}
	checkpoint, err := index.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreWorkspaceCallGraphIndex(checkpoint, map[CallGraphScope][]CallGraphDependency{scope: deps})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, ok := restored.Snapshot(scope, deps)
	if !ok {
		t.Fatal("restored fence graph missing")
	}
	want, err := graph.ForCompilationPlan(scope)
	if err != nil || !reflect.DeepEqual(want, snapshot) {
		t.Fatal("checkpoint changed fence facts", err)
	}
	id := callGraphNodeIDByName(t, graph, "Orders")
	for _, invalid := range []string{"", "Relaxed", "unknown"} {
		bad := graph.clone()
		bad.effects[id][0].AtomicOrder = invalid
		if validWorkspaceGraph(bad, deps) {
			t.Fatalf("cached fence accepted order %q", invalid)
		}
	}
}
