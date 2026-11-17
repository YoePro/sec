package sema

import (
	"os"
	"reflect"
	"slices"
	"testing"
)

// Unsafe authorization and safe wrapping preserve the canonical known/unknown
// domains and causes. The returned summary must not expose mutable graph data.
// Rules: rules/memory/unsafe.md §§13(3–6),15(1–5);
// rules/analysis/call_graph.md — "Same-stack effect transfer".
func TestUnsafeCanonicalEffects(t *testing.T) {
	data, err := os.ReadFile("../../testdata/analysis/unsafe_effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errs := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errs, nil)
	graph := a.CallGraph()
	id := func(name string) CallableID { return callGraphNodeIDByName(t, graph, name) }
	for _, name := range []string{"VolatileLeaf", "SafeWrapper", "RecursiveEffects", "MixedEffects"} {
		facts := a.TrustProvenance(id(name))
		if !slices.Contains(facts.Effects.Effects, SummaryMayAccessVolatile) {
			t.Fatalf("%s lost volatile effect: %+v", name, facts)
		}
		if (name == "VolatileLeaf") != facts.CallerFacingUnsafe {
			t.Fatalf("%s effect changed caller safety", name)
		}
		if name == "MixedEffects" && !slices.Contains(facts.Effects.UnknownEffects, SummaryMayIO) {
			t.Fatal("open target erased independent effect uncertainty")
		}
		if !reflect.DeepEqual(facts.Effects, graph.DomainSummary(id(name))) {
			t.Fatalf("%s diverged from canonical summary", name)
		}
	}
	for _, name := range []string{"PlainUnsafe", "PlainSafe"} {
		facts := a.TrustProvenance(id(name))
		if len(facts.Effects.Effects) != 0 || len(facts.Effects.UnknownEffects) != 0 {
			t.Fatalf("%s invented effects from safety: %+v", name, facts)
		}
	}
	foreign := a.TrustProvenance(id("ForeignWrapper"))
	if len(foreign.Effects.UnknownEffects) != 9 {
		t.Fatalf("foreign uncertainty incomplete: %+v", foreign.Effects)
	}
	foreign.Effects.UnknownEffects[0] = "changed"
	if slices.Contains(a.TrustProvenance(id("ForeignWrapper")).Effects.UnknownEffects, SummaryEffect("changed")) {
		t.Fatal("unknown domains alias analyzer state")
	}
	// Every §15(2) domain must flow through an unsafe declaration and its safe
	// wrapper. These are canonical producer facts, not new source attributes.
	// Publish to the analyzer-owned graph, not the detached public snapshot.
	graph = a.callGraph
	leaf := id("VolatileLeaf")
	node, _ := graph.Node(leaf)
	for _, kind := range []EffectKind{EffectMayPanicExplicit, EffectMayAllocate, EffectMayBlock, EffectMaySuspend, EffectMaySpawn, EffectMayIO, EffectMayAccessVolatile, EffectMayMutateExternalState, EffectMayUseNondeterministicInput} {
		graph.addEffect(leaf, EffectSite{Kind: kind, Source: node.Declaration})
	}
	all := a.TrustProvenance(id("SafeWrapper"))
	if len(all.Effects.Effects) != 9 || all.CallerFacingUnsafe {
		t.Fatalf("unsafe suppressed or changed summary: %+v", all)
	}
	all.Effects.Effects[0] = "changed"
	all.Effects.Causes[0].Fact = "changed"
	all.Effects.Causes[0].Path[0].Caller = "changed"
	if all.Effects.Causes[0].Effect != nil {
		all.Effects.Causes[0].Effect.Kind = "changed"
	}
	again := a.TrustProvenance(id("SafeWrapper"))
	if !reflect.DeepEqual(again.Effects, graph.DomainSummary(id("SafeWrapper"))) {
		t.Fatal("consumer mutated canonical effect facts")
	}
	for _, cause := range again.Effects.Causes {
		if cause.Fact == "changed" || len(cause.Path) > 0 && cause.Path[0].Caller == "changed" || cause.Effect != nil && cause.Effect.Kind == "changed" {
			t.Fatal("cause aliases graph state")
		}
	}
}
