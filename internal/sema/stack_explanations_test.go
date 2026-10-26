package sema

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
)

// Reports consume canonical recursive summaries at independent semantic/machine
// levels, retaining SCC membership, entry contribution, attached proof source,
// unknown boundaries and independent major contributors without summing them.
// Rules: rules/analysis/stack_analysis.md — "Stack cause paths", "Partial information", "OptionalInsight", "Stack analysis levels".
func TestStackExplanations(t *testing.T) {
	graph, frames, ids := recursiveStackInputs(t)
	proofs := []*StackRecursionDepthProof{
		recursiveStackProof(t, graph, ids["A"], StackMeasurementSemantic, "", big.NewInt(2)),
		recursiveStackProof(t, graph, ids["A"], StackMeasurementMachine, "plan", big.NewInt(2)),
	}
	semantic := ComposeSemanticStackSummariesWithBoundaryContracts(graph, frames, "", StackBoundaryContracts{Recursion: proofs})
	machine, err := ComposeMachineStackSummariesWithBoundaryContracts(graph, frames, "plan", StackBoundaryContracts{Recursion: proofs})
	if err != nil {
		t.Fatal(err)
	}
	var store StackSummaryStore
	for _, summary := range semantic {
		if err := store.RecordSemantic(summary); err != nil {
			t.Fatal(err)
		}
	}
	for _, summary := range machine {
		if err := store.RecordMachine(summary); err != nil {
			t.Fatal(err)
		}
	}
	beforeSemantic, beforeMachine := store.SemanticSummaries(), store.MachineSummaries()
	reports := store.SemanticExplanations(graph, proofs)
	machines := store.MachineExplanations(graph, proofs)
	report := stackExplanationNamed(t, reports, ids["A"], "")
	machineReport := stackExplanationNamed(t, machines, ids["A"], "plan")
	rendered := FormatStackExplanation(graph, report)
	for _, want := range []string{"Stack recursive_stack::A", "recursion SCC", "depth: UpperBoundDepth(2)", "depth-proof.sec:1:1", "major independent frames (not a simultaneous total)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in report: %s", want, rendered)
		}
	}
	for _, result := range []StackExplanation{report, machineReport} {
		if len(result.Recursion) != 1 {
			t.Fatalf("missing responsible SCC: %+v", result)
		}
		recursion := result.Recursion[0]
		count, known := recursion.Depth.Count()
		if !recursion.HasVerifiedDepth || !known || count.Cmp(big.NewInt(2)) != 0 || recursion.DepthSource.File != "depth-proof.sec" || len(recursion.Members) != 3 || recursion.Entry != ids["A"] {
			t.Fatalf("lost depth/source/component: %+v", recursion)
		}
		if recursion.Contribution.String() != result.Maximum.String() {
			t.Fatal("entry contribution belongs to another summary scope")
		}
		if len(result.MaximumCause) == 0 || len(result.MajorContributors) != 3 || result.MajorContributors[0].Callable != ids["C"] {
			t.Fatalf("lost maximum path or ranked contributors: %+v", result)
		}
		sum := new(big.Int)
		for index, frame := range result.MajorContributors {
			bytes, finite := frame.Frame.Bytes()
			if !finite {
				t.Fatal("known frame became unknown")
			}
			sum.Add(sum, bytes)
			if index > 0 {
				previous, _ := result.MajorContributors[index-1].Frame.Bytes()
				if previous.Cmp(bytes) < 0 {
					t.Fatal("major contributors are not descending")
				}
			}
		}
		maximum, _ := result.Maximum.Bytes()
		if maximum.Cmp(sum) == 0 {
			t.Fatal("report confused independent contributors with simultaneous recursive demand")
		}
	}
	if report.Level != StackMeasurementSemantic || machineReport.Level != StackMeasurementMachine {
		t.Fatal("measurement levels collapsed")
	}
	semanticBytes, _ := report.Maximum.Bytes()
	machineBytes, _ := machineReport.Maximum.Bytes()
	if machineBytes.Cmp(new(big.Int).Mul(semanticBytes, big.NewInt(2))) != 0 {
		t.Fatal("machine/semantic report costs mixed")
	}
	unknown := stackExplanationNamed(t, reports, ids["Callback"], "")
	if unknown.Maximum.Kind() != StackBoundUnknown || len(unknown.Evidence.UnknownCauses) == 0 || len(unknown.Evidence.UnknownPaths) == 0 || len(unknown.Recursion) != 1 || unknown.Recursion[0].HasVerifiedDepth {
		t.Fatalf("unknown recursive boundary erased: %+v", unknown)
	}
	// The store's stable namespace/insertion order and evidence normalization
	// make reporting independent of summary, contributor and proof order.
	var reordered StackSummaryStore
	for i := len(semantic) - 1; i >= 0; i-- {
		summary := semantic[i]
		summary.Evidence = cloneStackEvidence(summary.Evidence)
		for l, r := 0, len(summary.Evidence.Contributors)-1; l < r; l, r = l+1, r-1 {
			summary.Evidence.Contributors[l], summary.Evidence.Contributors[r] = summary.Evidence.Contributors[r], summary.Evidence.Contributors[l]
		}
		if err := reordered.RecordSemantic(summary); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(reports, reordered.SemanticExplanations(graph, []*StackRecursionDepthProof{proofs[1], proofs[0]})) {
		t.Fatal("report depends on insertion/evidence/proof order")
	}
	// All returned slices and arbitrary-precision count accessors are detached.
	report.MaximumCause[0].Detail = "changed"
	report.MajorContributors[0].Callable = "changed"
	report.Recursion[0].Members[0] = "changed"
	count, _ := report.Recursion[0].Depth.Count()
	count.SetInt64(999)
	if !reflect.DeepEqual(beforeSemantic, store.SemanticSummaries()) || !reflect.DeepEqual(beforeMachine, store.MachineSummaries()) {
		t.Fatal("report mutation escaped into summaries")
	}
	fresh := stackExplanationNamed(t, store.SemanticExplanations(graph, proofs), ids["A"], "")
	count, _ = fresh.Recursion[0].Depth.Count()
	if count.Cmp(big.NewInt(2)) != 0 || fresh.Recursion[0].Members[0] == "changed" {
		t.Fatal("report mutation escaped into graph/proofs")
	}
	// Proof attachment is exact: a cloned snapshot or other level cannot borrow
	// an otherwise plausible source/depth attestation.
	stale := stackExplanationNamed(t, store.SemanticExplanations(graph.clone(), proofs), ids["A"], "")
	wrongLevel := stackExplanationNamed(t, store.MachineExplanations(graph, proofs[:1]), ids["A"], "plan")
	var otherPlan StackSummaryStore
	copied, _ := store.Machine(ids["A"], "plan")
	copied.CompilationPlanID = "other"
	if err := otherPlan.RecordMachine(copied); err != nil {
		t.Fatal(err)
	}
	wrongPlan := stackExplanationNamed(t, otherPlan.MachineExplanations(graph, proofs), ids["A"], "other")
	for _, result := range []StackExplanation{stale, wrongLevel, wrongPlan} {
		if result.Recursion[0].HasVerifiedDepth || result.Recursion[0].DepthSource.File != "" {
			t.Fatal("mismatched proof gained a depth source")
		}
	}
}

// Independent plans retain their own classifications and source paths, even
// when no graph/depth metadata is available. Unknown is not Unbounded.
// Rules: rules/analysis/stack_analysis.md — "Stack cause paths", "CompilationPlan dependence", "Partial information".
func TestStackExplanationScopesAndTies(t *testing.T) {
	var store StackSummaryStore
	frame, _ := NewExactStackBound(big.NewInt(8))
	for _, plan := range []string{"first", "second"} {
		maximum := UnknownStackBound()
		if plan == "second" {
			maximum = UnboundedStackBound()
		}
		summary := SemanticStackSummary{Callable: "root", CompilationPlanID: plan, OwnFrame: frame, TransitiveMaximum: maximum, MaximumCause: []StackCauseStep{{Callable: "root", Detail: plan}}, Evidence: StackEvidence{Contributors: []StackFrameContribution{{Callable: "b", Frame: frame}, {Callable: "a", Frame: frame}}}}
		if err := store.RecordSemantic(summary); err != nil {
			t.Fatal(err)
		}
	}
	reports := store.SemanticExplanations(nil, nil)
	if len(reports) != 2 || reports[0].CompilationPlanID != "first" || reports[0].Maximum.Kind() != StackBoundUnknown || reports[1].Maximum.Kind() != StackBoundUnbounded {
		t.Fatal("plan/result classification lost", reports)
	}
	for _, report := range reports {
		if report.MajorContributors[0].Callable != "a" || len(report.Recursion) != 0 || report.MaximumCause[0].Detail != report.CompilationPlanID {
			t.Fatal("tie/path/graph-free report mismatch", report)
		}
	}
	if (*StackSummaryStore)(nil).SemanticExplanations(nil, nil) != nil || (*StackSummaryStore)(nil).MachineExplanations(nil, nil) != nil {
		t.Fatal("nil store invented reports")
	}
}

// stackExplanationNamed locates one exact reporting scope for assertions.
// Rules: rules/analysis/stack_analysis.md — "CompilationPlan dependence".
func stackExplanationNamed(t *testing.T, reports []StackExplanation, id CallableID, plan string) StackExplanation {
	t.Helper()
	for _, report := range reports {
		if report.Callable == id && report.CompilationPlanID == plan {
			return report
		}
	}
	t.Fatalf("missing report for %s/%s", id, plan)
	return StackExplanation{}
}
