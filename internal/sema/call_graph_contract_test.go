package sema

import (
	"math/big"
	"os"
	"reflect"
	"sort"
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
