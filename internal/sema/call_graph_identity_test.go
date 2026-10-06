package sema

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/analysis/call_graph.md — "Callable node identity", "Call-site
// identity", "Incremental tests", "One graph per `CompilationPlan`".
func TestCallGraphStableSnapshotAndPlanIdentity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_identity_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyze := func(source string) *CallGraph {
		t.Helper()
		parsed := parser.New(lexer.NewWithFile(source, "identity.sec"))
		program := parsed.ParseProgram()
		if errs := parsed.Errors(); len(errs) != 0 {
			t.Fatal(errs)
		}
		analyzer := NewAnalyzer()
		if errs := analyzer.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		return analyzer.CallGraph()
	}
	original := analyze(string(data))
	shiftedSource := "// unrelated formatting\n\n" + strings.ReplaceAll(string(data), "    ", "        ")
	shiftedSource = strings.Replace(shiftedSource, "module main", "module main\nfn Unrelated() void {}\n", 1)
	shiftedSource = strings.Replace(shiftedSource, "fn RecursiveA() void { RecursiveB() }\nfn RecursiveB() void { RecursiveA() }", "fn RecursiveB() void { RecursiveA() }\nfn RecursiveA() void { RecursiveB() }", 1)
	shifted := analyze(shiftedSource)
	chooseIDs := []CallableID{}
	for _, node := range original.Nodes() {
		later, exists := shifted.Node(node.ID)
		if !exists || later.Name != node.Name || later.Kind != node.Kind {
			t.Fatal("unrelated edit changed declaration/lambda/defer identity", node)
		}
		if later.Declaration.Line == node.Declaration.Line {
			t.Fatal("navigation did not refresh", node)
		}
		if node.Name == "Choose" {
			chooseIDs = append(chooseIDs, node.ID)
		}
		firstSites, laterSites := original.Outgoing(node.ID), shifted.Outgoing(node.ID)
		if len(firstSites) != len(laterSites) {
			t.Fatal("unrelated edit changed outgoing calls", node)
		}
		for i, site := range firstSites {
			later := laterSites[i]
			if site.ID != later.ID || !reflect.DeepEqual(site.Targets, later.Targets) || !reflect.DeepEqual(site.TargetSet, later.TargetSet) {
				t.Fatal("unrelated edit changed call-site identity or targets", site, later)
			}
			if site.Source.Line == later.Source.Line {
				t.Fatal("call-site source location is stale")
			}
		}
	}
	if len(chooseIDs) != 2 || chooseIDs[0] == chooseIDs[1] {
		t.Fatal("overloads collapsed")
	}
	mainID := callGraphNodeIDByName(t, original, "main")
	nodeIDs := func(nodes []CallableNode) []CallableID {
		ids := []CallableID{}
		for _, node := range nodes {
			ids = append(ids, node.ID)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids
	}
	if original.Roots()[0].ID != shifted.Roots()[0].ID || !reflect.DeepEqual(nodeIDs(original.ReachableFrom(original.Roots()[0].ID)), nodeIDs(shifted.ReachableFrom(shifted.Roots()[0].ID))) {
		t.Fatal("unrelated edit changed root reachability")
	}
	if !reflect.DeepEqual(nodeIDs(original.SameStackSCC(callGraphNodeIDByName(t, original, "RecursiveA"))), nodeIDs(shifted.SameStackSCC(callGraphNodeIDByName(t, shifted, "RecursiveA")))) {
		t.Fatal("recursive component changed")
	}
	changed := analyze(strings.Replace(string(data), "return value }", "return value + 1 }", 1))
	if !changed.EffectSummary(mainID).MayPanic || original.EffectSummary(mainID).MayPanic {
		t.Fatal("stable IDs retained stale callee facts")
	}
	for _, node := range original.Nodes() {
		if _, exists := changed.Node(node.ID); !exists {
			t.Fatal("callee body edit changed declaration identity", node)
		}
	}

	linux := CallGraphScope{"main", "linux-amd64", "identity-model-v2"}
	arm := CallGraphScope{"main", "linux-arm64", "identity-model-v2"}
	first, err := original.ForCompilationPlan(linux)
	if err != nil {
		t.Fatal(err)
	}
	other, err := original.ForCompilationPlan(arm)
	if err != nil {
		t.Fatal(err)
	}
	// Each frontend-selected source universe retains its own reachability.
	variant := analyze(strings.Replace(string(data), "fn main() void { Work(1) }", "fn main() void { RecursiveA() }", 1))
	variantPlan, err := variant.ForCompilationPlan(arm)
	if err != nil {
		t.Fatal(err)
	}
	variantReachable := nodeIDs(variantPlan.ReachableFrom(variantPlan.Roots()[0].ID))
	variantWork, ok := variantPlan.CallableInScope(callGraphNodeIDByName(t, variant, "Work"))
	if !ok {
		t.Fatal("variant lost Work declaration")
	}
	for _, id := range variantReachable {
		if id == variantWork {
			t.Fatal("inactive call became reachable across plan variants")
		}
	}
	if len(variantReachable) != 3 {
		t.Fatal("variant must reach exactly main and recursive pair", variantReachable)
	}
	later, err := shifted.ForCompilationPlan(linux)
	if err != nil {
		t.Fatal(err)
	}
	if first.CompilationScope() != linux || other.CompilationScope() != arm {
		t.Fatal("missing graph scope")
	}
	for _, node := range first.Nodes() {
		if _, exists := other.Node(node.ID); exists {
			t.Fatal("callable ID crossed plans")
		}
		if _, exists := later.Node(node.ID); !exists {
			t.Fatal("scoped ID changed after unrelated edit")
		}
	}
	for _, site := range first.sites {
		if other.siteIDs[site.ID] {
			t.Fatal("call-site ID crossed plans")
		}
	}
	boundMain, ok := first.CallableInScope(mainID)
	if !ok {
		t.Fatal("unbound semantic declaration could not resolve into plan graph")
	}
	if again, ok := first.CallableInScope(boundMain); !ok || again != boundMain {
		t.Fatal("identity double binding")
	}
	if !reflect.DeepEqual(nodeIDs(first.ReachableFrom(first.Roots()[0].ID)), nodeIDs(later.ReachableFrom(later.Roots()[0].ID))) {
		t.Fatal("scoped reachability changed after unrelated edit")
	}
	if rebound, err := first.ForCompilationPlan(arm); rebound != nil || err == nil {
		t.Fatal("concrete graph relabelled into another plan")
	}
	if missing, err := original.ForCompilationPlan(CallGraphScope{}); missing != nil || err == nil {
		t.Fatal("incomplete plan admitted")
	}
	unchanged, err := first.ForCompilationPlan(linux)
	if err != nil || !reflect.DeepEqual(first, unchanged) {
		t.Fatal("same-plan binding not idempotent")
	}
	unchanged.sites[0].Targets[0] = "mutated"
	if first.sites[0].Targets[0] == "mutated" || original.sites[0].Targets[0] == "mutated" {
		t.Fatal("bound snapshot shares target storage")
	}

	// Scope-aware workspace storage preserves same-plan IDs, rebuilds changed
	// callees and keeps unrelated concrete plans independent.
	deps := []CallGraphDependency{{"source", "identity.sec", "v1"}}
	var index WorkspaceCallGraphIndex
	for _, scope := range []CallGraphScope{linux, arm} {
		lease, err := index.Begin(scope, deps)
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := index.Publish(lease, original); !ok || err != nil {
			t.Fatal(ok, err)
		}
	}
	if graph, ok := index.Snapshot(linux, deps); !ok || !reflect.DeepEqual(first, graph) {
		t.Fatal("workspace did not bind canonical graph")
	}
	changedDeps := []CallGraphDependency{{"source", "identity.sec", "v2"}}
	lease, err := index.Begin(linux, changedDeps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(lease, changed); !ok || err != nil {
		t.Fatal(ok, err)
	}
	warm, ok := index.Snapshot(linux, changedDeps)
	if !ok || !warm.EffectSummary(boundMain).MayPanic {
		t.Fatal("changed body retained stale reachability/effect facts")
	}
	checkpoint, err := index.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreWorkspaceCallGraphIndex(checkpoint, map[CallGraphScope][]CallGraphDependency{linux: changedDeps})
	if err != nil {
		t.Fatal(err)
	}
	roundtrip, ok := restored.Snapshot(linux, changedDeps)
	if !ok || !reflect.DeepEqual(warm, roundtrip) {
		t.Fatal("plan-qualified identities changed after restore")
	}
}

// Rules: rules/analysis/call_graph.md — "Callable node identity";
// rules/corrections/applied/types-callable-model-correction-20260816.md — "Required correction".
func TestCallGraphDeclarationSignatureIdentity(t *testing.T) {
	base := Function{Name: "Read", Module: "main", Token: lexer.Token{File: "identity.sec", Line: 1, Column: 1},
		Parameters: []FunctionParameter{{Name: "value", Type: Type{Name: "int", Kind: IntType}}}, ReturnType: Type{Name: "void", Kind: VoidType}}
	stable := base
	stable.Token.Line = 99
	stable.Token.Column = 17
	stable.CompilerKnownID = "CKM-ITERATOR-NEXT"
	stable.ReceiverMutable = true // Inferred body demand is not declaration identity.
	stable.Parameters = append([]FunctionParameter(nil), base.Parameters...)
	stable.Parameters[0].Name = "renamed"
	if callableID(base) != callableID(stable) {
		t.Fatal("navigation/body tags or parameter labels changed identity")
	}
	variants := []Type{
		{Name: "Packet", Module: "first", Kind: StructType},
		{Name: "Packet", Module: "second", Kind: StructType},
		{Kind: ReferenceType, Element: &Type{Name: "Packet", Module: "first", Kind: StructType}},
		{Kind: ReferenceType, Element: &Type{Name: "Packet", Module: "second", Kind: StructType}},
		{Kind: ReferenceType, ReferenceMutable: true, Element: &Type{Name: "Packet", Module: "first", Kind: StructType}},
		{Name: "int", Kind: IntType, Unit: "cm"},
		{Name: "int", Kind: IntType, Unit: "mm"},
	}
	ids := map[CallableID]bool{}
	for _, typ := range variants {
		function := base
		function.Parameters = append([]FunctionParameter(nil), base.Parameters...)
		function.Parameters[0].Type = typ
		id := callableID(function)
		if ids[id] {
			t.Fatal("distinct nominal/reference/unit signatures collapsed", typ)
		}
		ids[id] = true
	}
	foreignA := base
	foreignA.Extern = true
	foreignA.ABI = "C"
	foreignA.LinkName = "read_a"
	foreignB := foreignA
	foreignB.LinkName = "read_b"
	if callableID(foreignA) == callableID(foreignB) {
		t.Fatal("foreign linkage collapsed")
	}
}

// Rules: rules/analysis/call_graph.md — "Reachability", "Inactive declarations";
// rules/control-flow/flowcontrol_if.md — statically unreachable branches.
func TestCallGraphIdentityUnreachableFunctionValue(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_identity_unreachable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), "dead-identity.sec"))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) == 0 {
		t.Fatal("unreachable statement must retain its semantic error")
	}
	graph := analyzer.CallGraph()
	mainID := callGraphNodeIDByName(t, graph, "main")
	if len(graph.Outgoing(mainID)) != 0 {
		t.Fatal("dead closed function-value call added an edge")
	}
	if reachable := graph.ReachableFrom(graph.Roots()[0].ID); len(reachable) != 1 || reachable[0].ID != mainID {
		t.Fatal("dead call added false lambda/recursive reachability", reachable)
	}
}
