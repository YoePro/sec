package sema

import (
	"math/big"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/analysis/call_graph.md — "Function values", "Interfaces",
// "Closed target set", "Open callable contract", "Incremental tests".
func TestCallGraphOpenAndJoinedTargets(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_open_targets_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), "open.sec"))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	graph := analyzer.CallGraph()
	site := func(name string) CallSite {
		t.Helper()
		calls := graph.Outgoing(callGraphNodeIDByName(t, graph, name))
		if len(calls) != 1 {
			t.Fatalf("%s calls: %#v", name, calls)
		}
		return calls[0]
	}
	names := func(call CallSite) []string {
		result := []string{}
		for _, target := range call.Targets {
			node, _ := graph.Node(target)
			result = append(result, node.Name)
		}
		sort.Strings(result)
		return result
	}
	closed, open, mixed, rebound, dynamic := site("Closed"), site("Open"), site("Mixed"), site("Rebound"), site("Dynamic")
	if !closed.TargetSet.IsClosed || !reflect.DeepEqual(names(closed), []string{"Risky", "Safe"}) {
		t.Fatal("closed union lost targets", closed)
	}
	if !rebound.TargetSet.IsClosed || !reflect.DeepEqual(names(rebound), []string{"Safe"}) {
		t.Fatal("straight-line rebinding lost exact target", rebound)
	}
	for _, call := range []CallSite{open, mixed, dynamic, site("Loop")} {
		contract := call.TargetSet.Contract
		if call.TargetSet.IsClosed || !call.TargetSet.HasOpenContract || contract == nil || contract.ID != call.TargetSet.OpenContract || contract.ReturnType == "" || contract.ABI != "Sec" {
			t.Fatal("missing validated public contract", call)
		}
		summary := graph.DomainSummary(call.Caller)
		if len(summary.UnknownEffects) != len(summaryEffects) {
			t.Fatal("open contract accidentally proved effect freedom", summary)
		}
		if !graph.EffectSummary(call.Caller).MayPanic || !graph.ArenaSummary(call.Caller).AllocationUnknown || !graph.BlockSummary(call.Caller).MayBlock {
			t.Fatal("legacy effect consumers lost unknown contract", call)
		}
	}
	if len(open.Targets) != 0 || !reflect.DeepEqual(names(mixed), []string{"Risky", "Safe"}) {
		t.Fatal("known-plus-open target projection", open, mixed)
	}
	if dynamic.Dispatch != CallDispatchInterface || len(dynamic.Targets) != 2 {
		t.Fatal("interface dispatch lost concrete targets", dynamic)
	}
	for _, target := range dynamic.Targets {
		node, _ := graph.Node(target)
		if node.ImplTarget != "First" && node.ImplTarget != "Second" {
			t.Fatal("implicit/nonconforming implementation entered targets", node)
		}
	}
	if open.TargetSet.OpenContract != mixed.TargetSet.OpenContract {
		t.Fatal("same validated function type acquired different contracts")
	}
	if len(open.TargetSet.Contract.Parameters) != 1 || open.TargetSet.Contract.Provenance != "validated-function-type" || dynamic.TargetSet.Contract.Provenance != "validated-interface-declaration" {
		t.Fatal("contract omitted source invocation facts")
	}
	// Optional stack guarantees are consumed for the same interface contract,
	// but an absent or wrong-plan guarantee cannot prove the open boundary.
	var contracts StackCallableContractStore
	boundBytes, _ := NewExactStackBound(big.NewInt(100))
	if _, usable := usableOpenStackContract(dynamic, &contracts, StackMeasurementSemantic, "plan"); usable {
		t.Fatal("missing stack guarantee became proof")
	}
	if err := contracts.Record(StackCallableContract{ID: dynamic.TargetSet.OpenContract, MeasurementLevel: StackMeasurementSemantic, CompilationPlanID: "plan", Maximum: boundBytes, NoReentry: true}); err != nil {
		t.Fatal(err)
	}
	if _, usable := usableOpenStackContract(dynamic, &contracts, StackMeasurementSemantic, "plan"); !usable {
		t.Fatal("verified interface stack contract was discarded")
	}
	if _, usable := usableOpenStackContract(dynamic, &contracts, StackMeasurementSemantic, "other"); usable {
		t.Fatal("interface stack guarantee crossed plans")
	}
	// Source graphs and concrete snapshots own their public facts independently.
	detached := graph.clone()
	detached.sites[0].TargetSet.KnownTargets[0] = "mutated"
	detachedOpen := detached.Outgoing(open.Caller)[0]
	detachedOpen.TargetSet.Contract.Parameters[0].TypeIdentity = "mutated"
	if site("Open").TargetSet.Contract.Parameters[0].TypeIdentity == "mutated" {
		t.Fatal("contract storage escaped snapshot")
	}
	scope := CallGraphScope{"main", "linux-amd64", "open-model-v3"}
	bound, err := graph.ForCompilationPlan(scope)
	if err != nil {
		t.Fatal(err)
	}
	scopedOpen, _ := bound.CallableInScope(open.Caller)
	scoped := bound.Outgoing(scopedOpen)[0]
	if scoped.TargetSet.OpenContract == open.TargetSet.OpenContract || scoped.TargetSet.Contract.ID != scoped.TargetSet.OpenContract {
		t.Fatal("contract crossed plan boundary")
	}
	var index WorkspaceCallGraphIndex
	deps := []CallGraphDependency{{"source", "open.sec", "v1"}}
	lease, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if committed, err := index.Publish(lease, graph); !committed || err != nil {
		t.Fatal(committed, err)
	}
	corrupted := graph.clone()
	for i := range corrupted.sites {
		if contract := corrupted.sites[i].TargetSet.Contract; contract != nil {
			contract.ReturnType = "wrong signature"
			break
		}
	}
	var rejected WorkspaceCallGraphIndex
	invalidLease, err := rejected.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if committed, err := rejected.Publish(invalidLease, corrupted); committed || err == nil {
		t.Fatal("mismatched contract signature published")
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
	if !ok || !reflect.DeepEqual(bound, snapshot) {
		t.Fatal("open contracts failed checkpoint roundtrip")
	}
}

// Rules: rules/analysis/call_graph.md — "Unknown callable contract",
// "Conservative unknown facts"; rules/errors/panic.md — §21 @noPanic.
func TestCallGraphOpenContractValidation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_open_targets_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), "invalid-open.sec"))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 2 {
		t.Fatal("must reject unsupported noPanic proof and noncallable invocation", errs)
	}
	graph := analyzer.CallGraph()
	if len(graph.Outgoing(callGraphNodeIDByName(t, graph, "Invalid"))) != 0 {
		t.Fatal("invalid contract acquired an invocation edge")
	}
	if calls := graph.Outgoing(callGraphNodeIDByName(t, graph, "Open")); len(calls) != 1 || calls[0].TargetSet.IsClosed {
		t.Fatal("valid invocation lost conservative contract", calls)
	}
}

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

// Rules: rules/analysis/call_graph.md — "Test roots", "Root reachability classes";
// rules/tooling/testing.md — §§5.7, 26, 27, 34.4.
func TestCallGraphSelectedTestRoots(t *testing.T) {
	file := "../../testdata/sema/call_graph_tests_valid_test.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), file))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	base := analyzer.CallGraph()
	if len(analyzer.ResolvedTests()) != 4 {
		t.Fatal("test discovery changed")
	}
	identities := map[string]TestIdentity{}
	for _, metadata := range analyzer.ResolvedTests() {
		identities[metadata.Name] = metadata.Identity
		node, exists := base.Node(testCallableID(metadata.Identity))
		if !exists || node.Kind != CallableBodyTest || node.Module != metadata.Identity.Module {
			t.Fatal("missing test body node", metadata, node)
		}
		if _, exists := analyzer.Functions()[metadata.Name]; exists {
			t.Fatal("test became source callable")
		}
		if len(base.RootsReaching(node.ID)) != 0 {
			t.Fatal("test leaked into production reachability")
		}
	}
	if len(base.Roots()) != 1 || base.Roots()[0].Kind != CallRootProgramEntry {
		t.Fatal("production roots changed", base.Roots())
	}
	scope := CallGraphScope{"main", "tests-linux", "test-root-model-v1"}
	selectGraph := func(names ...string) *CallGraph {
		t.Helper()
		selected := []TestIdentity{}
		for _, name := range names {
			selected = append(selected, identities[name])
		}
		graph, err := analyzer.CallGraphForTestPlan(scope, selected)
		if err != nil {
			t.Fatal(err)
		}
		return graph
	}
	first := selectGraph("first")
	if len(first.Roots()) != 1 || first.Roots()[0].Kind != CallRootTestEntry || first.Roots()[0].Scope != scope {
		t.Fatal("wrong test roots", first.Roots())
	}
	root := first.Roots()[0]
	reachable := map[string]bool{}
	for _, node := range first.ReachableFrom(root.ID) {
		reachable[node.Name] = true
	}
	if !reachable["first"] || !reachable["Shared"] || !reachable["defer"] || reachable["main"] || reachable["OnlyProduction"] || reachable["OnlySecond"] || reachable["second"] {
		t.Fatal("selection leaked or lost helper/cleanup", reachable)
	}
	if !first.EffectSummary(root.Node).MayPanic || len(first.DomainSummary(root.Node).Causes) == 0 {
		t.Fatal("test effects lost")
	}
	calls := first.Outgoing(root.Node)
	if len(calls) != 2 || calls[0].Execution != CallExecutionDeferred {
		t.Fatal("test call attribution lost", calls)
	}
	cleanup := first.Outgoing(calls[0].Targets[0])
	if len(cleanup) != 1 || cleanup[0].Caller == root.Node {
		t.Fatal("cleanup not attributed to synthetic body")
	}
	empty := selectGraph("empty")
	if len(empty.Roots()) != 1 || len(empty.ReachableFrom(empty.Roots()[0].ID)) != 1 {
		t.Fatal("empty test did not retain entry")
	}
	second := selectGraph("second")
	hasLambda := false
	for _, node := range second.ReachableFrom(second.Roots()[0].ID) {
		hasLambda = hasLambda || node.Kind == CallableBodyNonCapturingLambda
	}
	if !hasLambda {
		t.Fatal("test-local lambda lost")
	}
	worker := selectGraph("worker")
	if len(worker.Roots()) != 2 || worker.Roots()[1].Kind != CallRootTaskEntry {
		t.Fatal("selected test's worker root missing", worker.Roots())
	}
	if len(selectGraph().Roots()) != 0 {
		t.Fatal("empty selection implicitly selected tests")
	}
	together := selectGraph("second", "first")
	if !reflect.DeepEqual(together, selectGraph("first", "second")) {
		t.Fatal("selection order affected graph")
	}
	otherScope := scope
	otherScope.CompilationPlan = "tests-arm"
	other, err := analyzer.CallGraphForTestPlan(otherScope, []TestIdentity{identities["first"]})
	if err != nil {
		t.Fatal(err)
	}
	if other.Roots()[0].ID == root.ID {
		t.Fatal("root identity lost compilation plan")
	}
	for _, selection := range [][]TestIdentity{{identities["first"], identities["first"]}, {{Module: "main", Path: []string{"unknown"}}}, {{Module: "main", Path: []string{"first", "child"}}}, {{Module: "other", Path: []string{"first"}}}, {{}}} {
		if graph, err := analyzer.CallGraphForTestPlan(scope, selection); graph != nil || err == nil {
			t.Fatal("bad selection admitted", selection)
		}
	}
	if graph, err := analyzer.CallGraphForTestPlan(CallGraphScope{}, nil); graph != nil || err == nil {
		t.Fatal("missing plan admitted")
	}
	first.sites[0].Targets[0] = "mutated"
	if !reflect.DeepEqual(base, analyzer.CallGraph()) || selectGraph("first").sites[0].Targets[0] == "mutated" {
		t.Fatal("test view mutated canonical facts")
	}

	// Selected roots persist only under their actual plan/model scope.
	graph := selectGraph("first", "worker")
	deps := []CallGraphDependency{{"source", file, "v1"}, {"test-selection", "main-tests", "first-worker"}}
	var index WorkspaceCallGraphIndex
	lease, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(lease, graph); !ok || err != nil {
		t.Fatal("test graph publication", ok, err)
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
	if !ok || !reflect.DeepEqual(graph, snapshot) {
		t.Fatal("test roots did not survive checkpoint")
	}
	if !reflect.DeepEqual(graph.Roots(), snapshot.Roots()) {
		t.Fatal("derived worker roots changed")
	}
	wrong, err := index.Begin(otherScope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(wrong, graph); ok || err == nil {
		t.Fatal("test roots published under another plan")
	}
	index.Invalidate("test-selection", "main-tests")
	if _, ok := index.Snapshot(scope, deps); ok {
		t.Fatal("changed selection retained roots")
	}
}

// Rules: rules/analysis/call_graph.md — "Unreachable code", "Test roots";
// rules/compiler/incremental_compilation.md — §§29–33.
func TestCallGraphTestPlanRejectsInvalidAnalysis(t *testing.T) {
	file := "../../testdata/sema/call_graph_tests_invalid_test.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), file))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := NewAnalyzer()
	if len(analyzer.Analyze(program)) == 0 {
		t.Fatal("invalid test fixture succeeded")
	}
	scope := CallGraphScope{"main", "test-plan", "test-root-model-v1"}
	if graph, err := analyzer.CallGraphForTestPlan(scope, []TestIdentity{{Module: "main", Path: []string{"broken"}}}); graph != nil || err == nil {
		t.Fatal("failed analysis released selected roots")
	}
}
