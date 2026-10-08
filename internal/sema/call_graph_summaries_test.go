package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Source facts and explicit producer facts share one domain propagation view.
// Rules: rules/analysis/call_graph.md — "Effect-analysis integration", "Same-stack effect transfer",
// "Spawn effect transfer", "Closed target set", "Conservative unknown facts";
// rules/analysis/effect_analysis.md — "Summary may-effects", "Fixed-point analysis".
func TestCallGraphEffectDomains(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_effect_domains_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	const file = "effect-domains.sec"
	parsed := parser.New(lexer.NewWithFile(string(data), file))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	graph := analyzer.CallGraph()
	id := func(name string) CallableID { return callGraphNodeIDByName(t, graph, name) }
	has := func(effects []SummaryEffect, wanted SummaryEffect) bool {
		for _, effect := range effects {
			if effect == wanted {
				return true
			}
		}
		return false
	}
	original := graph.clone()
	top := graph.DomainSummary(id("Top"))
	for _, domain := range []SummaryEffect{SummaryMayPanic, SummaryMayAccessVolatile, SummaryMayUseNondeterministicInput} {
		if !has(top.Effects, domain) {
			t.Fatal("missing transitive domain", domain, top)
		}
	}
	if len(top.UnknownEffects) != 0 || len(top.DirectCauses) != 0 {
		t.Fatal("invented direct effect or uncertainty", top)
	}
	for _, cause := range top.Causes {
		if len(cause.Path) != 2 || cause.Path[0].Caller != id("Top") || cause.Path[1].Caller != id("Nested") || cause.Path[1].Targets[0] != cause.Callable {
			t.Fatal("incomplete source cause path", cause)
		}
		for _, site := range cause.Path {
			if site.Execution != CallExecutionSynchronous || site.ID == "" || site.Source.Line == 0 || site.Dispatch == "" {
				t.Fatal("missing path metadata", cause)
			}
		}
	}
	spawner := graph.DomainSummary(id("Spawner"))
	if !has(spawner.Effects, SummaryMaySpawn) || has(spawner.Effects, SummaryMayUseNondeterministicInput) {
		t.Fatal("spawn body merged into caller context", spawner)
	}
	if !has(graph.DomainSummary(id("Worker")).Effects, SummaryMayUseNondeterministicInput) {
		t.Fatal("spawned body lost its own effects")
	}
	if !has(graph.DomainSummary(id("RecursiveA")).Effects, SummaryMayAccessVolatile) {
		t.Fatal("SCC propagation lost effects")
	}
	for _, name := range []string{"Pure", "DeadBranch"} {
		summary := graph.DomainSummary(id(name))
		if len(summary.Effects) != 0 || len(summary.UnknownEffects) != 0 || len(summary.Causes) != 0 {
			t.Fatal("dead or pure body gained effects", name, summary)
		}
	}
	for _, name := range []string{"Opaque", "ForeignCaller", "Foreign"} {
		summary := graph.DomainSummary(id(name))
		if len(summary.UnknownEffects) != len(summaryEffects) || len(summary.Causes) == 0 {
			t.Fatal("unknown effects became empty proof", name, summary)
		}
	}
	if !reflect.DeepEqual(original, graph.clone()) {
		t.Fatal("consumer mutated canonical facts")
	}
	// Mutating one result or a direct cause must not mutate another published
	// cause, target coverage, graph snapshot, or later query.
	saved := graph.DomainSummary(id("Top"))
	changed := graph.DomainSummary(id("Top"))
	changed.Causes[0].Path[0].Targets[0] = "mutated"
	changed.Causes[0].Path[0].TargetSet.KnownTargets[0] = "mutated"
	changed.Effects[0] = "mutated"
	if !reflect.DeepEqual(saved, graph.DomainSummary(id("Top"))) {
		t.Fatal("published evidence aliases graph")
	}
	// Producer-supplied facts have the same propagation and set semantics as
	// source facts. This does not claim source syntax or import verification.
	leaf := id("Pure")
	node, _ := graph.Node(leaf)
	for _, kind := range []EffectKind{EffectMayIO, EffectMaySuspend, EffectMayMutateExternalState, EffectMaySpawn} {
		graph.addEffect(leaf, EffectSite{Kind: kind, Source: node.Declaration})
		graph.addEffect(leaf, EffectSite{Kind: kind, Source: node.Declaration})
	}
	graph.addArenaEffect(leaf, ArenaEffectSite{Kind: ArenaEffectAllocate, Source: node.Declaration, MayAllocate: true})
	graph.addArenaEffect(leaf, ArenaEffectSite{Kind: ArenaEffectReset, Source: node.Declaration})
	graph.addBlockEffect(leaf, BlockEffectSite{Kind: BlockEffectOperation, Source: node.Declaration})
	produced := graph.DomainSummary(leaf)
	for _, domain := range []SummaryEffect{SummaryMayIO, SummaryMaySuspend, SummaryMayMutateExternalState, SummaryMaySpawn, SummaryMayAllocate, SummaryMayBlock} {
		if !has(produced.Effects, domain) {
			t.Fatal("canonical domain not consumed", domain, produced)
		}
	}
	if len(produced.Effects) != 6 || len(produced.Causes) != 6 || len(produced.UnknownEffects) != 0 {
		t.Fatal("effects failed idempotence or arena reset became allocation", produced)
	}
	produced.DirectCauses[0].Fact = "changed"
	if produced.Causes[0].Fact == "changed" {
		t.Fatal("direct and transitive causes share ownership")
	}
	// A joined closed set unions all alternatives; opening it retains every
	// known cause while independently requiring a covering effects contract.
	joined := graph.clone()
	site := joined.Outgoing(id("Top"))[0]
	site.Targets = []CallableID{id("Nested"), leaf}
	site.TargetSet.KnownTargets = nil
	for body, target := range joined.bodyNodes {
		if target == id("Nested") || target == leaf {
			site.TargetSet.KnownTargets = append(site.TargetSet.KnownTargets, body)
		}
	}
	for i := range joined.sites {
		if joined.sites[i].ID == site.ID {
			joined.sites[i] = site
		}
	}
	if len(joined.DomainSummary(id("Top")).Effects) != 9 {
		t.Fatal("closed alternatives not unioned")
	}
	for i := range joined.sites {
		if joined.sites[i].ID == site.ID {
			joined.sites[i].TargetSet.IsClosed = false
			joined.sites[i].TargetSet.OpenContract = "open"
		}
	}
	opened := joined.DomainSummary(id("Top"))
	if len(opened.Effects) != 9 || len(opened.UnknownEffects) != 9 {
		t.Fatal("open contract erased known contributions or fabricated guarantees", opened)
	}
	reversed := joined.clone()
	for i, j := 0, len(reversed.sites)-1; i < j; i, j = i+1, j-1 {
		reversed.sites[i], reversed.sites[j] = reversed.sites[j], reversed.sites[i]
	}
	for callable, effects := range reversed.effects {
		for i, j := 0, len(effects)-1; i < j; i, j = i+1, j-1 {
			effects[i], effects[j] = effects[j], effects[i]
		}
		reversed.effects[callable] = effects
	}
	if !reflect.DeepEqual(opened, reversed.DomainSummary(id("Top"))) {
		t.Fatal("registration order changed summary")
	}
	graph.addEffect(leaf, EffectSite{Kind: "future-domain", Source: node.Declaration})
	if len(graph.DomainSummary(leaf).UnknownEffects) != 9 {
		t.Fatal("unclassified future effect silently omitted")
	}
	if len(graph.DomainSummary("missing").Causes) != 0 {
		t.Fatal("unknown callable has fabricated evidence")
	}
	var absent *CallGraph
	if len(absent.DomainSummary(leaf).Causes) != 0 {
		t.Fatal("nil graph has fabricated evidence")
	}
}
