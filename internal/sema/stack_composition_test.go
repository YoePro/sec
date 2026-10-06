package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestStackCompositionDirectPaths consumes Sema's canonical reachable graph
// and independent supplied frame facts, checking sequential/branch maxima,
// nested sums, unreachable paths and unsupported boundaries at both levels.
// Rules: rules/analysis/stack_analysis.md — "Call-path composition", "Control-flow composition",
// "Call graph ownership", "Acyclic call graphs", and "Unknown recursion depth".
func TestStackCompositionDirectPaths(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/stack_composition_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errs, nil)
	graph := analyzer.CallGraph()
	var frames StackSummaryStore
	for _, node := range graph.Nodes() {
		size := int64(100)
		switch node.Name {
		case "Small", "Middle", "Twin", "Counter.Read":
			size = 500
		case "Large":
			size = 800
		}
		bound, _ := NewExactStackBound(big.NewInt(size))
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, OwnFrame: bound}); err != nil {
			t.Fatal(err)
		}
		if err := frames.RecordMachine(MachineStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: bound}); err != nil {
			t.Fatal(err)
		}
	}
	before := frames.SemanticSummaries()
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
	for name, size := range map[string]int64{"Sequential": 900, "Equal": 600, "Method": 600, "Branch": 900, "Middle": 1300, "Nested": 1400, "Small": 500, "Large": 800} {
		result := byName[name]
		bytes, finite := result.TransitiveMaximum.Bytes()
		if !finite || bytes.Cmp(big.NewInt(size)) != 0 {
			t.Fatalf("%s: %s, want %d", name, result.TransitiveMaximum, size)
		}
		want := StackBoundUpperBound
		if name == "Small" || name == "Large" {
			want = StackBoundExact
		}
		if result.TransitiveMaximum.Kind() != want {
			t.Fatal("unsafe exactness claim", name, result)
		}
	}
	for _, name := range []string{"Recursive", "MutualA", "MutualB", "CallsRecursive", "Foreign", "CallsForeign", "Indirect", "Arithmetic"} {
		result := byName[name]
		if result.TransitiveMaximum.Kind() != StackBoundUnknown || len(result.MaximumCause) == 0 {
			t.Fatal("unsupported call gained finite/unbounded proof", name, result)
		}
		if _, finite := result.OwnFrame.Bytes(); !finite {
			t.Fatal("known frame erased", name)
		}
	}
	if len(byName["Nested"].MaximumCause) != 3 {
		t.Fatal("nested cause path lost", byName["Nested"])
	}
	for i, summary := range machine {
		if summary.TransitiveMaximum != semantic[i].TransitiveMaximum || summary.CompilationPlanID != "plan" {
			t.Fatal("machine composition differs", summary)
		}
	}
	if !reflect.DeepEqual(before, frames.SemanticSummaries()) {
		t.Fatal("composition changed input facts")
	}
	// Equivalent canonical registration orders must yield identical maxima
	// and representative causes, including shared DAG subpaths.
	reordered := graph.clone()
	for left, right := 0, len(reordered.nodeOrder)-1; left < right; left, right = left+1, right-1 {
		reordered.nodeOrder[left], reordered.nodeOrder[right] = reordered.nodeOrder[right], reordered.nodeOrder[left]
	}
	for left, right := 0, len(reordered.sites)-1; left < right; left, right = left+1, right-1 {
		reordered.sites[left], reordered.sites[right] = reordered.sites[right], reordered.sites[left]
	}
	for _, effects := range reordered.effects {
		for left, right := 0, len(effects)-1; left < right; left, right = left+1, right-1 {
			effects[left], effects[right] = effects[right], effects[left]
		}
	}
	if !reflect.DeepEqual(semantic, ComposeSemanticStackSummaries(reordered, &frames, "")) {
		t.Fatal("graph order changed results")
	}
	if _, err := ComposeMachineStackSummaries(graph, &frames, ""); err == nil {
		t.Fatal("machine plan not required")
	}
	for _, summary := range ComposeSemanticStackSummaries(graph, &frames, "other-plan") {
		if summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("wrong-plan frame used", summary)
		}
	}
	large := byName["Large"].Callable
	machineFrame, _ := NewExactStackBound(big.NewInt(1600))
	if err := frames.RecordMachine(MachineStackSummary{Callable: large, CompilationPlanID: "plan", OwnFrame: machineFrame}); err != nil {
		t.Fatal(err)
	}
	changedMachine, err := ComposeMachineStackSummaries(graph, &frames, "plan")
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range changedMachine {
		if summary.Callable == byName["Sequential"].Callable {
			bytes, finite := summary.TransitiveMaximum.Bytes()
			if !finite || bytes.Cmp(big.NewInt(1700)) != 0 {
				t.Fatal("machine frames substituted with semantic evidence", summary)
			}
		}
	}
	if !reflect.DeepEqual(semantic, ComposeSemanticStackSummaries(graph, &frames, "")) {
		t.Fatal("machine frame update changed semantic composition")
	}
}

// TestStackCompositionUnreachableFacts confirms that the shared call graph
// excludes Sema-proven dead calls while the owning validity error is preserved.
// It does not authorize compilation of the invalid source.
// Rules: rules/analysis/stack_analysis.md — "Control-flow composition";
// rules/analysis/call_graph.md — reachable calls.
func TestStackCompositionUnreachableFacts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/stack_composition_unreachable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "unreachable statement") {
		t.Fatal("owning unreachable error lost", errs)
	}
	graph := analyzer.CallGraph()
	var frames StackSummaryStore
	for _, node := range graph.Nodes() {
		bound, _ := NewExactStackBound(big.NewInt(100))
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, OwnFrame: bound}); err != nil {
			t.Fatal(err)
		}
		if node.Name == "Dead" && len(graph.Outgoing(node.ID)) != 0 {
			t.Fatal("dead call leaked into canonical graph")
		}
	}
	for _, summary := range ComposeSemanticStackSummaries(graph, &frames, "") {
		if summary.TransitiveMaximum != summary.OwnFrame {
			t.Fatal("dead call changed frame composition", summary)
		}
	}
}

// TestStackCompositionBoundAlgebra checks arbitrary-precision sums, uncertainty
// and proof quality without conflating overflow or recursion with unboundedness.
// Rules: rules/analysis/stack_analysis.md — "Call-path composition", "UpperBound",
// "Unknown", and "Unbounded".
func TestStackCompositionBoundAlgebra(t *testing.T) {
	large := new(big.Int).Lsh(big.NewInt(1), 128)
	own, _ := NewExactStackBound(large)
	callee, _ := NewUpperStackBound(large)
	result := composeStackCall(own, callee)
	bytes, finite := result.Bytes()
	want := new(big.Int).Lsh(big.NewInt(1), 129)
	if !finite || bytes.Cmp(want) != 0 || result.Kind() != StackBoundUpperBound {
		t.Fatal("sum overflow or proof quality", result)
	}
	for _, missing := range []StackBound{UnknownStackBound(), UnboundedStackBound()} {
		if composeStackCall(own, missing).Kind() != StackBoundUnknown {
			t.Fatal("missing call-path proof invented")
		}
		if composeStackCall(missing, callee).Kind() != StackBoundUnknown {
			t.Fatal("missing frame proof invented")
		}
	}
	if stackCompositionStronger(UnknownStackBound(), UnboundedStackBound()) {
		t.Fatal("existing unbounded proof weakened")
	}
	if len(ComposeSemanticStackSummaries(nil, nil, "")) != 0 {
		t.Fatal("missing graph produced summaries")
	}
}
