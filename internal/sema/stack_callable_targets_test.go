package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"
)

// stackCallableInputs reads real Sec callable flow and supplies independent
// verified frame facts; it does not derive layout from types or source names.
// Rules: rules/analysis/stack_analysis.md — "Closed indirect-call target sets",
// "ResolvedLayout is authoritative", and "Stack analysis levels".
func stackCallableInputs(t *testing.T) (*CallGraph, *StackSummaryStore, map[string]CallableID) {
	t.Helper()
	source, err := os.ReadFile("../../testdata/sema/stack_callable_targets_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errs, nil)
	graph := analyzer.CallGraph()
	frames := &StackSummaryStore{}
	ids := map[string]CallableID{}
	for _, node := range graph.Nodes() {
		ids[node.Name] = node.ID
		size := int64(100)
		switch node.Name {
		case "Small":
			size = 500
		case "Large":
			size = 800
		case "lambda":
			size = 300
		}
		semantic, _ := NewExactStackBound(big.NewInt(size))
		machine, _ := NewExactStackBound(big.NewInt(2 * size))
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, OwnFrame: semantic}); err != nil {
			t.Fatal(err)
		}
		if err := frames.RecordMachine(MachineStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: machine}); err != nil {
			t.Fatal(err)
		}
	}
	return graph, frames, ids
}

// TestStackClosedCallableTargets verifies real named, joined, nested, lambda
// and capturing-closure target sets at both independent measurement levels.
// Rules: rules/analysis/stack_analysis.md — "Closed indirect-call target sets",
// "Call-path composition", "Open calls without stack contracts", and "Determinism".
func TestStackClosedCallableTargets(t *testing.T) {
	graph, frames, ids := stackCallableInputs(t)
	sites := graph.Outgoing(ids["Joined"])
	if len(sites) != 1 || !sites[0].TargetSet.IsClosed || len(sites[0].Targets) != 2 {
		t.Fatal("fixture did not produce a canonical closed joined call", sites)
	}
	beforeSemantic, beforeMachine := frames.SemanticSummaries(), frames.MachineSummaries()
	semantic := ComposeSemanticStackSummaries(graph, frames, "")
	machine, err := ComposeMachineStackSummaries(graph, frames, "plan")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[CallableID]SemanticStackSummary{}
	for _, summary := range semantic {
		byID[summary.Callable] = summary
	}
	for name, size := range map[string]int64{"Named": 900, "Joined": 900, "JoinedNested": 1000, "Lambda": 1200, "Capturing": 400} {
		result := byID[ids[name]]
		bytes, finite := result.TransitiveMaximum.Bytes()
		if !finite || bytes.Cmp(big.NewInt(size)) != 0 || result.TransitiveMaximum.Kind() != StackBoundUpperBound {
			t.Fatalf("%s = %s, want upper %d", name, result.TransitiveMaximum, size)
		}
	}
	for _, name := range []string{"JoinedRecursive", "Open"} {
		if byID[ids[name]].TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("unsupported target gained proof", name)
		}
	}
	if len(byID[ids["JoinedNested"]].MaximumCause) != 3 || len(byID[ids["Capturing"]].MaximumCause) != 2 {
		t.Fatal("indirect maximum cause lost")
	}
	for _, summary := range machine {
		semanticBytes, finite := byID[summary.Callable].TransitiveMaximum.Bytes()
		if finite {
			machineBytes, machineFinite := summary.TransitiveMaximum.Bytes()
			if !machineFinite || machineBytes.Cmp(semanticBytes.Mul(semanticBytes, big.NewInt(2))) != 0 {
				t.Fatal("machine/semantic facts mixed", summary)
			}
		} else if summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("machine uncertainty lost", summary)
		}
	}
	if !reflect.DeepEqual(beforeSemantic, frames.SemanticSummaries()) || !reflect.DeepEqual(beforeMachine, frames.MachineSummaries()) {
		t.Fatal("composition mutated frame inputs")
	}
	reversed := graph.clone()
	for index := range reversed.sites {
		site := &reversed.sites[index]
		for left, right := 0, len(site.Targets)-1; left < right; left, right = left+1, right-1 {
			site.Targets[left], site.Targets[right] = site.Targets[right], site.Targets[left]
		}
		for left, right := 0, len(site.TargetSet.KnownTargets)-1; left < right; left, right = left+1, right-1 {
			site.TargetSet.KnownTargets[left], site.TargetSet.KnownTargets[right] = site.TargetSet.KnownTargets[right], site.TargetSet.KnownTargets[left]
		}
	}
	if !reflect.DeepEqual(semantic, ComposeSemanticStackSummaries(reversed, frames, "")) {
		t.Fatal("target order changed proof or cause")
	}
	var tied StackSummaryStore
	for _, summary := range beforeSemantic {
		if summary.Callable == ids["Small"] {
			summary.OwnFrame = byID[ids["Large"]].OwnFrame
		}
		if err := tied.RecordSemantic(summary); err != nil {
			t.Fatal(err)
		}
	}
	tiedResult := ComposeSemanticStackSummaries(graph, &tied, "")
	if !reflect.DeepEqual(tiedResult, ComposeSemanticStackSummaries(reversed, &tied, "")) {
		t.Fatal("equal-cost target order changed representative cause")
	}
	// One missing target poisons the maximum even though another target remains
	// finite. Independently known caller frames and machine facts survive.
	if err := frames.RecordSemantic(SemanticStackSummary{Callable: ids["Small"]}); err != nil {
		t.Fatal(err)
	}
	for _, summary := range ComposeSemanticStackSummaries(graph, frames, "") {
		if summary.Callable == ids["Joined"] && (summary.TransitiveMaximum.Kind() != StackBoundUnknown || summary.OwnFrame.Kind() != StackBoundExact) {
			t.Fatal("finite known subset used as complete proof", summary)
		}
	}
	freshMachine, _ := ComposeMachineStackSummaries(graph, frames, "plan")
	if !reflect.DeepEqual(machine, freshMachine) {
		t.Fatal("semantic missing target changed machine results")
	}
}

// TestStackClosedTargetCoverage rejects partial/open/inconsistent target facts
// without inventing a finite contract from their still-known finite targets.
// Rules: rules/analysis/stack_analysis.md — "Closed indirect-call target sets",
// "Open callable contracts", "Open calls without stack contracts", and "Physical stack domains".
func TestStackClosedTargetCoverage(t *testing.T) {
	graph, frames, ids := stackCallableInputs(t)
	tests := []struct {
		name   string
		change func(*CallGraph, *CallSite)
	}{
		{"open", func(_ *CallGraph, site *CallSite) { site.TargetSet.IsClosed = false }},
		{"open-contract", func(_ *CallGraph, site *CallSite) { site.TargetSet.HasOpenContract = true }},
		{"contract-identity", func(_ *CallGraph, site *CallSite) { site.TargetSet.OpenContract = "open-contract" }},
		{"empty", func(_ *CallGraph, site *CallSite) { site.Targets = nil; site.TargetSet.KnownTargets = nil }},
		{"missing-target", func(_ *CallGraph, site *CallSite) { site.Targets = site.Targets[:1] }},
		{"missing-body", func(g *CallGraph, site *CallSite) { delete(g.bodyNodes, site.TargetSet.KnownTargets[0]) }},
		{"missing-node", func(g *CallGraph, site *CallSite) { delete(g.nodes, site.Targets[0]) }},
		{"duplicate-target", func(_ *CallGraph, site *CallSite) { site.Targets[1] = site.Targets[0] }},
		{"execution-boundary", func(_ *CallGraph, site *CallSite) { site.Execution = CallExecutionSpawnThread }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := graph.clone()
			for index := range changed.sites {
				if changed.sites[index].Caller == ids["Joined"] {
					test.change(changed, &changed.sites[index])
					break
				}
			}
			for _, result := range ComposeSemanticStackSummaries(changed, frames, "") {
				if result.Callable == ids["Joined"] && result.TransitiveMaximum.Kind() != StackBoundUnknown {
					t.Fatal("invalid coverage gained a proof", result)
				}
			}
		})
	}
}
