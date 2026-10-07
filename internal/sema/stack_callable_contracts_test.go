package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"
)

// TestStackOpenCallableContracts enriches real canonical source graphs with
// explicit producer-supplied open contract facts; no Sec contract syntax is
// claimed. All missing facts remain unknown rather than closing the world.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts",
// "Open calls without stack contracts", "Partial information", and "CompilationPlan dependence".
func TestStackOpenCallableContracts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/stack_open_callable_contracts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, nil)
	graph := analyzer.CallGraph()
	ids := map[string]CallableID{}
	var frames StackSummaryStore
	for _, node := range graph.Nodes() {
		ids[node.Name] = node.ID
		size := int64(100)
		if node.Name == "Small" {
			size = 500
		}
		if node.Name == "Large" {
			size = 800
		}
		semantic, _ := NewExactStackBound(big.NewInt(size))
		machine, _ := NewExactStackBound(big.NewInt(2 * size))
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, OwnFrame: semantic}); err != nil {
			t.Fatal(err)
		}
		if err := frames.RecordMachine(MachineStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: machine}); err != nil {
			t.Fatal(err)
		}
		index := 0
		for _, effect := range graph.effects[node.ID] {
			if effect.Kind != EffectMayPanicUnknownCallee {
				continue
			}
			contract := CallableContractID("A")
			if node.Name == "TwoOpen" && index > 0 {
				contract = "B"
			}
			// The ordinary producer now retains this invocation. Attach the
			// test producer's verified guarantee identity to that same site.
			for i := range graph.sites {
				site := &graph.sites[i]
				if site.Caller == node.ID && site.Source == effect.Source {
					site.TargetSet.OpenContract = contract
					if site.TargetSet.Contract != nil {
						site.TargetSet.Contract.ID = contract
					}
				}
			}
			index++
		}
	}
	for i := range graph.sites {
		if graph.sites[i].Caller == ids["Joined"] {
			graph.sites[i].TargetSet.IsClosed = false
			graph.sites[i].TargetSet.HasOpenContract = true
			graph.sites[i].TargetSet.OpenContract = "A"
		}
	}
	beforeGraph, beforeFrames := graph.clone(), frames.SemanticSummaries()
	var contracts StackCallableContractStore
	bound, _ := NewUpperStackBound(big.NewInt(1000))
	if err := contracts.Record(StackCallableContract{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: bound, NoReentry: true}); err != nil {
		t.Fatal(err)
	}
	get := func(results []SemanticStackSummary, name string) SemanticStackSummary {
		t.Helper()
		for _, summary := range results {
			if summary.Callable == ids[name] {
				return summary
			}
		}
		t.Fatal("missing summary", name)
		return SemanticStackSummary{}
	}
	results := ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts)
	for name, count := range map[string]int64{"Joined": 1100, "Open": 1100, "Outer": 1200} {
		result := get(results, name)
		bytes, finite := result.TransitiveMaximum.Bytes()
		if !finite || bytes.Cmp(big.NewInt(count)) != 0 || result.TransitiveMaximum.Kind() != StackBoundUpperBound {
			t.Fatal("open contract not composed soundly", name, result)
		}
	}
	if len(get(results, "Joined").Evidence.Contributors) != 3 {
		t.Fatal("known target evidence erased")
	}
	for _, name := range []string{"TwoOpen", "Panic"} {
		if get(results, name).TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("unrelated opaque/panic boundary suppressed", name)
		}
	}
	if !reflect.DeepEqual(beforeGraph, graph.clone()) || !reflect.DeepEqual(beforeFrames, frames.SemanticSummaries()) {
		t.Fatal("consumer mutated graph/frame facts")
	}
	for _, absent := range []string{"", "wrong-plan"} {
		for _, summary := range ComposeSemanticStackSummaries(graph, &frames, absent) {
			if summary.Callable == ids["Joined"] && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
				t.Fatal("absent contracts became closed targets")
			}
		}
	}
	machine, err := ComposeMachineStackSummariesWithContracts(graph, &frames, "plan", &contracts)
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range machine {
		if summary.Callable == ids["Open"] && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("semantic guarantee used as machine evidence")
		}
	}
	machineBound, _ := NewExactStackBound(big.NewInt(2000))
	if err := contracts.Record(StackCallableContract{ID: "A", MeasurementLevel: StackMeasurementMachine, CompilationPlanID: "plan", Maximum: machineBound, NoReentry: true}); err != nil {
		t.Fatal(err)
	}
	machine, err = ComposeMachineStackSummariesWithContracts(graph, &frames, "plan", &contracts)
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range machine {
		if summary.Callable == ids["Open"] {
			bytes, finite := summary.TransitiveMaximum.Bytes()
			if !finite || bytes.Cmp(big.NewInt(2200)) != 0 {
				t.Fatal("machine contract not independent", summary)
			}
		}
	}
	if err := contracts.Record(StackCallableContract{ID: "B", MeasurementLevel: StackMeasurementSemantic, Maximum: bound, NoReentry: true}); err != nil {
		t.Fatal(err)
	}
	if bytes, finite := get(ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts), "TwoOpen").TransitiveMaximum.Bytes(); !finite || bytes.Cmp(big.NewInt(1100)) != 0 {
		t.Fatal("sequential open calls summed instead of taking maximum")
	}
	for _, contract := range []StackCallableContract{
		{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: bound},
		{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: UnknownStackBound(), NoReentry: true},
		{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: UnboundedStackBound(), NoReentry: true},
	} {
		if err := contracts.Record(contract); err != nil {
			t.Fatal(err)
		}
		result := get(ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts), "Joined")
		if result.TransitiveMaximum.Kind() != StackBoundUnknown || len(result.Evidence.Contributors) != 3 || len(result.Evidence.UnknownCauses) == 0 {
			t.Fatal("unusable contract lost uncertainty/known targets", result)
		}
	}
	conflicting, _ := NewUpperStackBound(big.NewInt(700))
	if err := contracts.Record(StackCallableContract{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: conflicting, NoReentry: true}); err != nil {
		t.Fatal(err)
	}
	if result := get(ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts), "Joined"); result.TransitiveMaximum.Kind() != StackBoundUnknown || len(result.Evidence.Contributors) != 3 {
		t.Fatal("contradictory contract became a finite proof", result)
	}
	originalSmall, _ := frames.Semantic(ids["Small"], "")
	looseSmall, _ := NewUpperStackBound(big.NewInt(900))
	if err := frames.RecordSemantic(SemanticStackSummary{Callable: ids["Small"], OwnFrame: looseSmall}); err != nil {
		t.Fatal(err)
	}
	if result := get(ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts), "Joined"); result.TransitiveMaximum.Kind() != StackBoundUnknown {
		t.Fatal("loose upper target hid another target's exact contract violation", result)
	}
	if err := frames.RecordSemantic(originalSmall); err != nil {
		t.Fatal(err)
	}
	zero, _ := NewExactStackBound(big.NewInt(0))
	if err := contracts.Record(StackCallableContract{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: zero, NoReentry: true}); err != nil {
		t.Fatal(err)
	}
	if result := get(ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts), "Open"); result.TransitiveMaximum.String() != "UpperBound(100)" {
		t.Fatal("explicit zero contract confused with absence", result)
	}
	large, _ := NewUpperStackBound(new(big.Int).Lsh(big.NewInt(1), 128))
	if err := contracts.Record(StackCallableContract{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: large, NoReentry: true}); err != nil {
		t.Fatal(err)
	}
	result := get(ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts), "Open")
	bytes, _ := result.TransitiveMaximum.Bytes()
	want, _ := large.Bytes()
	want.Add(want, big.NewInt(100))
	if bytes.Cmp(want) != 0 {
		t.Fatal("open contract arithmetic overflow")
	}
	for _, mutate := range []func(*CallSite){
		func(site *CallSite) { site.TargetSet.HasOpenContract = false },
		func(site *CallSite) { site.TargetSet.OpenContract = "missing" },
		func(site *CallSite) { site.TargetSet.KnownTargets[0] = "missing-body" },
		func(site *CallSite) { site.Targets[0] = site.Targets[1] },
		func(site *CallSite) { site.Execution = CallExecutionSpawnThread },
		func(site *CallSite) { site.Dispatch = CallDispatchForeign },
	} {
		modified := graph.clone()
		for i := range modified.sites {
			if modified.sites[i].Caller == ids["Joined"] {
				mutate(&modified.sites[i])
				break
			}
		}
		if result := get(ComposeSemanticStackSummariesWithContracts(modified, &frames, "", &contracts), "Joined"); result.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("uncovered/open-mismatched call acquired finite proof", result)
		}
	}
	reordered := graph.clone()
	for l, r := 0, len(reordered.nodeOrder)-1; l < r; l, r = l+1, r-1 {
		reordered.nodeOrder[l], reordered.nodeOrder[r] = reordered.nodeOrder[r], reordered.nodeOrder[l]
	}
	for l, r := 0, len(reordered.sites)-1; l < r; l, r = l+1, r-1 {
		reordered.sites[l], reordered.sites[r] = reordered.sites[r], reordered.sites[l]
	}
	if !reflect.DeepEqual(ComposeSemanticStackSummariesWithContracts(graph, &frames, "", &contracts), ComposeSemanticStackSummariesWithContracts(reordered, &frames, "", &contracts)) {
		t.Fatal("open contract results depend on registration order")
	}
}

// TestStackCallableContractIdentity rejects malformed publication and preserves
// exact level/plan variants without mutable proof-count ownership.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts" and "CompilationPlan dependence".
func TestStackCallableContractIdentity(t *testing.T) {
	var store StackCallableContractStore
	for _, invalid := range []StackCallableContract{{}, {ID: "A"}, {ID: "A", MeasurementLevel: StackMeasurementMachine}} {
		if err := store.Record(invalid); err == nil {
			t.Fatal("invalid contract published")
		}
	}
	bound, _ := NewExactStackBound(big.NewInt(500))
	contract := StackCallableContract{ID: "A", MeasurementLevel: StackMeasurementSemantic, Maximum: bound, NoReentry: true}
	if err := store.Record(contract); err != nil {
		t.Fatal(err)
	}
	contract.Maximum = UnknownStackBound()
	got, exists := store.Lookup("A", StackMeasurementSemantic, "")
	if !exists || got.Maximum != bound {
		t.Fatal("producer changed published guarantee")
	}
	bytes, _ := got.Maximum.Bytes()
	bytes.SetInt64(0)
	fresh, _ := store.Lookup("A", StackMeasurementSemantic, "")
	if count, finite := fresh.Maximum.Bytes(); !finite || count.Cmp(big.NewInt(500)) != 0 {
		t.Fatal("mutable count escaped contract store")
	}
	for _, key := range []stackCallableContractKey{{"B", StackMeasurementSemantic, ""}, {"A", StackMeasurementMachine, "plan"}, {"A", StackMeasurementSemantic, "other"}} {
		if _, exists := store.Lookup(key.id, key.level, key.plan); exists {
			t.Fatal("identity fallback")
		}
	}
	var absent *StackCallableContractStore
	if err := absent.Record(contract); err == nil {
		t.Fatal("nil store accepted publication")
	}
	if _, exists := absent.Lookup("A", StackMeasurementSemantic, ""); exists {
		t.Fatal("nil store supplied a proof")
	}
}
