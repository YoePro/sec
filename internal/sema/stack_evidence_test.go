package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"
)

// TestStackPartialEvidence retains prefixes, alternative contributors and all
// uncertainty boundaries while totals remain Unknown at independent levels.
// Rules: rules/analysis/stack_analysis.md — "Partial information", "Stack cause paths",
// "Closed indirect-call target sets", "CompilationPlan dependence", and "Determinism".
func TestStackPartialEvidence(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/stack_partial_evidence_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, nil)
	graph := analyzer.CallGraph()
	var frames StackSummaryStore
	ids := map[string]CallableID{}
	for _, node := range graph.Nodes() {
		ids[node.Name] = node.ID
		if node.Name == "Missing" || node.Name == "Gap" {
			continue
		}
		size := int64(100)
		switch node.Name {
		case "Middle":
			size = 500
		case "Opaque":
			size = 1536
		case "Leaf":
			size = 800
		}
		semantic, _ := NewUpperStackBound(big.NewInt(size))
		machine, _ := NewExactStackBound(big.NewInt(size * 2))
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, OwnFrame: semantic}); err != nil {
			t.Fatal(err)
		}
		if err := frames.RecordMachine(MachineStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: machine}); err != nil {
			t.Fatal(err)
		}
	}
	semantic := ComposeSemanticStackSummaries(graph, &frames, "")
	machine, err := ComposeMachineStackSummaries(graph, &frames, "plan")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]SemanticStackSummary{}
	for _, summary := range semantic {
		node, _ := graph.Node(summary.Callable)
		byName[node.Name] = summary
	}
	root := byName["Root"]
	var nestedPrefix []StackFrameContribution
	for _, path := range root.Evidence.UnknownPaths {
		if path.Boundary.Callable == ids["Opaque"] {
			nestedPrefix = path.Frames
		}
	}
	if root.TransitiveMaximum.Kind() != StackBoundUnknown || len(nestedPrefix) != 3 {
		t.Fatal("unknown path lost known prefix", root)
	}
	for i, name := range []string{"Root", "Middle", "Opaque"} {
		if nestedPrefix[i].Callable != ids[name] {
			t.Fatal("wrong representative prefix", root.Evidence)
		}
	}
	if len(root.Evidence.Contributors) != 4 || len(root.Evidence.UnknownCauses) != 4 || len(root.Evidence.UnknownPaths) != 4 {
		t.Fatal("alternative frame or unknown boundary erased", root.Evidence)
	}
	if bytes, _ := byName["Opaque"].Evidence.KnownPrefix[0].Frame.Bytes(); bytes.Cmp(big.NewInt(1536)) != 0 {
		t.Fatal("rulebook's known 1536-byte prefix lost")
	}
	for _, name := range []string{"Runtime", "Missing", "RecursiveA", "RecursiveB", "Joined"} {
		if byName[name].TransitiveMaximum.Kind() != StackBoundUnknown || len(byName[name].Evidence.UnknownCauses) == 0 {
			t.Fatal("unknown evidence missing", name, byName[name])
		}
	}
	if len(byName["Runtime"].Evidence.UnknownCauses) != 2 || len(byName["Runtime"].Evidence.Contributors) != 2 {
		t.Fatal("opaque/panic boundary hid known sibling", byName["Runtime"])
	}
	if len(byName["Missing"].Evidence.KnownPrefix) != 0 || len(byName["Missing"].Evidence.Contributors) != 1 {
		t.Fatal("missing own frame became a known prefix or erased callee", byName["Missing"])
	}
	if len(byName["Joined"].Evidence.Contributors) != 4 || len(byName["RecursiveA"].Evidence.Contributors) != 2 {
		t.Fatal("closed target/SCC contributors lost", byName["Joined"], byName["RecursiveA"])
	}
	if len(byName["Diamond"].Evidence.UnknownPaths) != 2 || len(byName["Diamond"].Evidence.Contributors) != 4 {
		t.Fatal("shared boundary duplicated paths or lost branch contributors", byName["Diamond"])
	}
	for _, path := range byName["Gap"].Evidence.UnknownPaths {
		if path.Boundary.Callable == ids["Opaque"] && (len(path.Frames) != 2 || path.Frames[0].Frame.Kind() != StackBoundUnknown || path.Frames[1].Frame.Kind() != StackBoundUpperBound) {
			t.Fatal("unknown frame gap erased later known prefix", path)
		}
	}
	budget, _ := NewStackBudget("domain", StackMeasurementSemantic, big.NewInt(100000))
	if result, err := budget.Compare("domain", StackMeasurementSemantic, root.TransitiveMaximum); err != nil || result != StackBudgetUnknown {
		t.Fatal("known prefix became a budget proof", result, err)
	}
	for i, summary := range machine {
		if (summary.TransitiveMaximum.Kind() == StackBoundUnknown) != (semantic[i].TransitiveMaximum.Kind() == StackBoundUnknown) || len(summary.Evidence.Contributors) != len(semantic[i].Evidence.Contributors) {
			t.Fatal("machine evidence scope differs", summary)
		}
		for j, frame := range summary.Evidence.Contributors {
			bytes, _ := frame.Frame.Bytes()
			semanticBytes, _ := semantic[i].Evidence.Contributors[j].Frame.Bytes()
			if frame.Frame.Kind() != StackBoundExact || bytes.Cmp(new(big.Int).Mul(semanticBytes, big.NewInt(2))) != 0 {
				t.Fatal("measurement levels mixed", frame)
			}
		}
	}
	reordered := graph.clone()
	for l, r := 0, len(reordered.nodeOrder)-1; l < r; l, r = l+1, r-1 {
		reordered.nodeOrder[l], reordered.nodeOrder[r] = reordered.nodeOrder[r], reordered.nodeOrder[l]
	}
	for l, r := 0, len(reordered.sites)-1; l < r; l, r = l+1, r-1 {
		reordered.sites[l], reordered.sites[r] = reordered.sites[r], reordered.sites[l]
	}
	for _, effects := range reordered.effects {
		for l, r := 0, len(effects)-1; l < r; l, r = l+1, r-1 {
			effects[l], effects[r] = effects[r], effects[l]
		}
	}
	if !reflect.DeepEqual(semantic, ComposeSemanticStackSummaries(reordered, &frames, "")) {
		t.Fatal("evidence depends on graph registration order")
	}
	for _, summary := range ComposeSemanticStackSummaries(graph, &frames, "other") {
		if len(summary.Evidence.Contributors) != 0 || len(summary.Evidence.KnownPrefix) != 0 {
			t.Fatal("wrong-plan frame leaked", summary)
		}
	}
	if err := frames.RecordSemantic(SemanticStackSummary{Callable: ids["Runtime"], OwnFrame: UnboundedStackBound()}); err != nil {
		t.Fatal(err)
	}
	for _, summary := range ComposeSemanticStackSummaries(graph, &frames, "") {
		if summary.Callable == ids["Runtime"] && (summary.TransitiveMaximum.Kind() != StackBoundUnbounded || len(summary.Evidence.UnknownCauses) != 2 || len(summary.Evidence.Contributors) != 1) {
			t.Fatal("stronger unbounded fact or independent uncertainty erased", summary)
		}
	}
}

// TestStackEvidenceSnapshots detaches nested evidence at publication, lookup and
// list boundaries, including separate semantic and machine snapshots.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results;
// rules/analysis/stack_analysis.md — "Partial information" and "Stack analysis levels".
func TestStackEvidenceSnapshots(t *testing.T) {
	frame, _ := NewExactStackBound(big.NewInt(1536))
	evidence := StackEvidence{
		KnownPrefix:   []StackFrameContribution{{Callable: "F", Frame: frame}},
		Contributors:  []StackFrameContribution{{Callable: "F", Frame: frame}},
		UnknownCauses: []StackCauseStep{{Detail: "unknown contract"}},
		UnknownPaths:  []StackUnknownPath{{Frames: []StackFrameContribution{{Callable: "F", Frame: frame}}, Boundary: StackCauseStep{Detail: "unknown contract"}}},
	}
	var store StackSummaryStore
	if err := store.RecordSemantic(SemanticStackSummary{Callable: "F", Evidence: evidence}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordMachine(MachineStackSummary{Callable: "F", CompilationPlanID: "plan", Evidence: evidence}); err != nil {
		t.Fatal(err)
	}
	beforeS, beforeM := store.SemanticSummaries(), store.MachineSummaries()
	mutate := func(value StackEvidence) {
		value.KnownPrefix[0].Callable = "changed"
		value.Contributors[0].Frame = UnknownStackBound()
		value.UnknownCauses[0].Detail = "changed"
		value.UnknownPaths[0].Frames[0].Callable = "changed"
		value.UnknownPaths[0].Boundary.Detail = "changed"
	}
	mutate(evidence)
	s, _ := store.Semantic("F", "")
	mutate(s.Evidence)
	m, _ := store.Machine("F", "plan")
	mutate(m.Evidence)
	mutate(store.SemanticSummaries()[0].Evidence)
	mutate(store.MachineSummaries()[0].Evidence)
	if !reflect.DeepEqual(beforeS, store.SemanticSummaries()) || !reflect.DeepEqual(beforeM, store.MachineSummaries()) {
		t.Fatal("mutable partial evidence escaped snapshot boundary")
	}
}
