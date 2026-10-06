package sema

import (
	"math/big"
	"reflect"
	"testing"
)

// TestStackSummaryLevels preserves the rulebook's valid semantic-upper/machine-
// exact pair, plan-specific results and localized unknown transitive demand.
// Rules: rules/analysis/stack_analysis.md — "Stack analysis levels",
// "CompilationPlan dependence", "Partial information", and "Summary model".
func TestStackSummaryLevels(t *testing.T) {
	semanticMaximum, _ := NewUpperStackBound(big.NewInt(8192))
	machineMaximum, _ := NewExactStackBound(big.NewInt(7344))
	frame, _ := NewExactStackBound(big.NewInt(1536))
	semantic := SemanticStackSummary{Callable: "Decode", OwnFrame: frame,
		TransitiveMaximum: semanticMaximum, MaximumCause: []StackCauseStep{{Detail: "semantic frame"}}}
	machine := MachineStackSummary{Callable: "Decode", CompilationPlanID: "linux/amd64",
		OwnFrame: frame, TransitiveMaximum: machineMaximum, MaximumCause: []StackCauseStep{{Detail: "backend frame"}}}
	var store StackSummaryStore
	if err := store.RecordSemantic(semantic); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordMachine(machine); err != nil {
		t.Fatal(err)
	}
	gotSemantic, ok := store.Semantic("Decode", "")
	if !ok || !reflect.DeepEqual(gotSemantic, semantic) {
		t.Fatal("machine erased semantic proof", gotSemantic)
	}
	gotMachine, ok := store.Machine("Decode", "linux/amd64")
	if !ok || !reflect.DeepEqual(gotMachine, machine) {
		t.Fatal("machine proof lost", gotMachine)
	}
	semantic.CompilationPlanID = "cortex-m"
	semantic.TransitiveMaximum = UnknownStackBound()
	semantic.MaximumCause[0].Detail = "unknown foreign stack contract"
	if err := store.RecordSemantic(semantic); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Machine("Decode", "cortex-m"); ok {
		t.Fatal("semantic result substituted for machine evidence")
	}
	machine.CompilationPlanID = "cortex-m"
	machine.TransitiveMaximum = UnboundedStackBound()
	if err := store.RecordMachine(machine); err != nil {
		t.Fatal(err)
	}
	gotMachine, ok = store.Machine("Decode", "linux/amd64")
	if !ok || gotMachine.TransitiveMaximum != machineMaximum {
		t.Fatal("another plan erased machine evidence")
	}
	gotSemantic, ok = store.Semantic("Decode", "cortex-m")
	if !ok || gotSemantic.OwnFrame != frame || gotSemantic.TransitiveMaximum.Kind() != StackBoundUnknown {
		t.Fatal("unknown maximum erased known frame", gotSemantic)
	}
	semantic.CompilationPlanID = ""
	semantic.TransitiveMaximum = UnknownStackBound()
	if err := store.RecordSemantic(semantic); err != nil {
		t.Fatal(err)
	}
	gotMachine, _ = store.Machine("Decode", "linux/amd64")
	if gotMachine.TransitiveMaximum != machineMaximum {
		t.Fatal("semantic refresh erased machine proof")
	}
	if _, ok := store.Semantic("Decode", "other-plan"); ok {
		t.Fatal("silent cross-plan semantic fallback")
	}
	if _, ok := store.Machine("Decode", ""); ok {
		t.Fatal("machine result without a plan")
	}
}

// TestStackSummarySnapshots keeps producer and consumer mutations outside the
// stored facts and makes ordering independent of publication order.
// Rules: rules/analysis/stack_analysis.md — "Summary model" and "Determinism";
// rules/compiler/compiler_analysis.md — immutable analysis results.
func TestStackSummarySnapshots(t *testing.T) {
	var store, reversed StackSummaryStore
	inputs := []SemanticStackSummary{
		{Callable: "Z", CompilationPlanID: "b", MaximumCause: []StackCauseStep{{Detail: "cause Z"}}},
		{Callable: "A", CompilationPlanID: "b", MaximumCause: []StackCauseStep{{Detail: "cause Ab"}}},
		{Callable: "A", MaximumCause: []StackCauseStep{{Detail: "cause A"}}},
	}
	for i, summary := range inputs {
		if err := store.RecordSemantic(summary); err != nil {
			t.Fatal(err)
		}
		summary.CompilationPlanID = "machine" + summary.CompilationPlanID
		if err := store.RecordMachine(MachineStackSummary(summary)); err != nil {
			t.Fatal(err)
		}
		reverse := inputs[len(inputs)-1-i]
		if err := reversed.RecordSemantic(reverse); err != nil {
			t.Fatal(err)
		}
		reverse.CompilationPlanID = "machine" + reverse.CompilationPlanID
		if err := reversed.RecordMachine(MachineStackSummary(reverse)); err != nil {
			t.Fatal(err)
		}
	}
	beforeSemantic, beforeMachine := store.SemanticSummaries(), store.MachineSummaries()
	if !reflect.DeepEqual(beforeSemantic, reversed.SemanticSummaries()) || !reflect.DeepEqual(beforeMachine, reversed.MachineSummaries()) {
		t.Fatal("publication order changed snapshots")
	}
	if beforeSemantic[0].Callable != "A" || beforeSemantic[0].CompilationPlanID != "" || beforeSemantic[1].CompilationPlanID != "b" {
		t.Fatal("snapshot order is not callable then plan", beforeSemantic)
	}
	inputs[0].MaximumCause[0].Detail = "producer edit"
	semantic, _ := store.Semantic("Z", "b")
	semantic.MaximumCause[0].Detail = "consumer edit"
	machine, _ := store.Machine("Z", "machineb")
	machine.MaximumCause[0].Detail = "machine consumer edit"
	semanticList, machineList := store.SemanticSummaries(), store.MachineSummaries()
	semanticList[0].MaximumCause[0].Detail = "semantic list edit"
	machineList[0].MaximumCause[0].Detail = "machine list edit"
	if !reflect.DeepEqual(beforeSemantic, store.SemanticSummaries()) || !reflect.DeepEqual(beforeMachine, store.MachineSummaries()) {
		t.Fatal("snapshot or input mutation changed stored facts")
	}
}

// TestStackSummaryIdentityValidation rejects absent machine plan identities and
// never publishes invalid entries or represents absent evidence as exact zero.
// Rules: rules/analysis/stack_analysis.md — "CompilationPlan dependence" and "Unknown".
func TestStackSummaryIdentityValidation(t *testing.T) {
	var store StackSummaryStore
	if err := store.RecordSemantic(SemanticStackSummary{}); err == nil {
		t.Fatal("missing callable accepted")
	}
	for _, invalid := range []MachineStackSummary{{}, {Callable: "F"}, {CompilationPlanID: "plan"}} {
		if err := store.RecordMachine(invalid); err == nil {
			t.Fatal("invalid machine identity accepted", invalid)
		}
	}
	if len(store.SemanticSummaries()) != 0 || len(store.MachineSummaries()) != 0 {
		t.Fatal("invalid entry published")
	}
	semantic, ok := store.Semantic("missing", "")
	if ok || semantic.OwnFrame.Kind() != StackBoundUnknown || semantic.TransitiveMaximum.Kind() != StackBoundUnknown {
		t.Fatal("missing semantic entry became a proof")
	}
	var absent *StackSummaryStore
	if absent.RecordSemantic(SemanticStackSummary{Callable: "F"}) == nil || absent.RecordMachine(MachineStackSummary{Callable: "F", CompilationPlanID: "plan"}) == nil {
		t.Fatal("nil store accepted publication")
	}
	if _, ok := absent.Semantic("F", ""); ok {
		t.Fatal("nil store semantic evidence")
	}
	if _, ok := absent.Machine("F", "plan"); ok {
		t.Fatal("nil store machine evidence")
	}
	if len(absent.SemanticSummaries()) != 0 || len(absent.MachineSummaries()) != 0 {
		t.Fatal("nil store has summaries")
	}
}
