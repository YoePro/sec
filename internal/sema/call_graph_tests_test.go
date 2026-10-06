package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

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
