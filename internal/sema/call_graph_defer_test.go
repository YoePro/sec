package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/analysis/call_graph.md — "`defer` bodies", "Same-stack execution",
// "Recursive `defer`", "Incremental tests"; rules/control-flow/defer.md — §§12–13, 18.
func TestCallGraphDefer(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_defer_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), "defer.sec"))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(program), nil)
	g := analyzer.CallGraph()
	named := map[string]CallableID{}
	for _, node := range g.Nodes() {
		if node.Kind == CallableBodyNamedFunction {
			named[node.Name] = node.ID
		}
	}
	after := g.Outgoing(named["After"])
	if len(after) != 2 || after[1].Caller != named["After"] || after[1].Execution != CallExecutionSynchronous {
		t.Fatal("caller was not restored after defer", after)
	}
	lambdaCalls := g.Outgoing(named["Lambda"])
	if len(lambdaCalls) != 1 {
		t.Fatal("lambda invocation missing", lambdaCalls)
	}
	lambdaID := lambdaCalls[0].Targets[0]
	cleanupCalls := g.Outgoing(lambdaID)
	if len(cleanupCalls) != 1 || cleanupCalls[0].Execution != CallExecutionDeferred {
		t.Fatal("defer did not belong to lambda invocation", cleanupCalls)
	}

	sites := g.Outgoing(named["Work"])
	if len(sites) != 2 {
		t.Fatal("expected one edge per defer including empty body", sites)
	}
	for i, site := range sites {
		if site.Execution != CallExecutionDeferred || site.Dispatch != CallDispatchGenerated || !site.TargetSet.IsClosed || len(site.Targets) != 1 {
			t.Fatal("lost cleanup relation or closed target", site)
		}
		body, ok := g.Node(site.Targets[0])
		if !ok || body.Kind != CallableBodyDefer || len(g.RootsReaching(body.ID)) != 1 {
			t.Fatal("missing reachable defer node", body)
		}
		calls := g.Outgoing(body.ID)
		if i == 0 && len(calls) != 0 {
			t.Fatal("empty body has calls", calls)
		}
		if i == 1 && (len(calls) != 1 || calls[0].Targets[0] != named["Cleanup"] || calls[0].Execution != CallExecutionSynchronous) {
			t.Fatal("wrong deferred-body caller", calls)
		}
	}
	if !g.EffectSummary(named["Work"]).MayPanic || len(g.EffectSummary(named["Work"]).DirectEffects) != 0 {
		t.Fatal("cleanup panic did not propagate separately")
	}
	domain := g.DomainSummary(named["Work"])
	if len(domain.UnknownEffects) != 0 {
		t.Fatal("known cleanup marked unknown", domain)
	}
	found := false
	for _, cause := range domain.Causes {
		if cause.Domain == SummaryMayPanic && len(cause.Path) == 2 && cause.Path[0].Execution == CallExecutionDeferred {
			found = true
		}
	}
	if !found {
		t.Fatal("missing deferred cause path", domain)
	}
	if !g.IsSameStackRecursive(named["Recursive"]) || len(g.SameStackSCC(named["Recursive"])) != 2 || len(g.CompleteExecutionSCC(named["Recursive"])) != 2 || g.IsInTaskSpawnCycle(named["Recursive"]) {
		t.Fatal("cleanup recursion misclassified")
	}
	if len(g.Outgoing(named["Repeated"])) != 1 {
		t.Fatal("loop registrations should share one source node")
	}
	// Existing allocation/blocking consumers must use the same cleanup edge.
	body := sites[1].Targets[0]
	g.addArenaEffect(body, ArenaEffectSite{Kind: ArenaEffectAllocate, Source: sites[1].Source, MayAllocate: true})
	g.addBlockEffect(body, BlockEffectSite{Kind: BlockEffectOperation, Source: sites[1].Source})
	if !g.ArenaSummary(named["Work"]).MayAllocate || !g.BlockSummary(named["Work"]).MayBlock {
		t.Fatal("cleanup effects lost")
	}
	var frames StackSummaryStore
	for _, node := range g.Nodes() {
		bound, _ := NewExactStackBound(big.NewInt(10))
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, OwnFrame: bound}); err != nil {
			t.Fatal(err)
		}
	}
	// Isolate graph stack composition from separately unknown runtime panic costs.
	stackGraph := g.clone()
	stackGraph.effects = map[CallableID][]EffectSite{}
	for _, summary := range ComposeSemanticStackSummaries(stackGraph, &frames, "") {
		if summary.Callable == named["Work"] {
			bytes, finite := summary.TransitiveMaximum.Bytes()
			if !finite || bytes.Int64() != 30 {
				t.Fatal("cleanup stack contribution lost", summary)
			}
		}
	}
	// Checkpoint validation accepts and preserves the new relation and node kind.
	deps := []CallGraphDependency{{"source", sites[0].Source.File, "v1"}}
	scope := CallGraphScope{"main", "test-plan", "defer-model-v1"}
	var index WorkspaceCallGraphIndex
	lease, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(lease, g); !ok || err != nil {
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
	expected, err := g.ForCompilationPlan(scope)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, ok := restored.Snapshot(scope, deps)
	if !ok || !reflect.DeepEqual(expected, snapshot) {
		t.Fatal("cleanup graph changed on restore")
	}
}

// Rules: rules/control-flow/defer.md — §§12, 18; rules/analysis/call_graph.md —
// "Semantic reachability" and "`defer` bodies".
func TestCallGraphDeferUnreachableAndInvalid(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_defer_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errs) == 0 {
		t.Fatal("invalid fixture succeeded")
	}
	rejectedNested := false
	for _, err := range errs {
		rejectedNested = rejectedNested || strings.Contains(err.Message, "defer is not allowed inside defer")
	}
	if !rejectedNested {
		t.Fatal("nested defer diagnostic missing", errs)
	}
	g := analyzer.CallGraph()
	deferred := 0
	for _, node := range g.Nodes() {
		if node.Kind != CallableBodyDefer {
			continue
		}
		deferred++
		if node.Declaration.Line == 5 && (len(g.Incoming(node.ID)) != 0 || len(g.Outgoing(node.ID)) != 0) {
			t.Fatal("dead cleanup contributed calls")
		}
	}
	if deferred != 2 {
		t.Fatal("dead defer should retain a node; nested invalid defer should not", deferred)
	}
}
