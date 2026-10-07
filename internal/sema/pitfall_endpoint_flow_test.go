package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Endpoint proofs compose through control flow and short-circuit evaluation;
// mutation and unrelated identities must never grant a suppression.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guards participate
// in pitfall reasoning", "Inclusive upper bound against collection length".
func TestPitfallEndpointFlow(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_endpoint_flow_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]PitfallAnalysisState{}
	for _, name := range []string{"Negated", "Disjunction", "NestedEndpoint", "BothNestedArms", "SplitPaths", "AssertGuard", "PanicGuard", "UnreachableGuard", "ShortCircuitSafe", "ShortCircuitOrSafe", "OppositeBranch", "OtherBinding", "ElementWrite", "WhileEndpointReturn", "UnitStep"} {
		want[name] = []PitfallAnalysisState{PitfallStateSuppressed}
	}
	for _, name := range []string{"ConditionalConjunction", "InnerBreak", "ShortCircuitUnsafe", "WrongCollection", "WhileEndpointBreak"} {
		want[name] = []PitfallAnalysisState{PitfallStateFinding}
	}
	for _, name := range []string{"Mutate", "MutableCall", "MutationOnEarlierIteration", "Growing", "Strided"} {
		want[name] = nil
	}
	want["BranchLocal"] = []PitfallAnalysisState{PitfallStateSuppressed, PitfallStateFinding}
	want["GuardOwnAccess"] = []PitfallAnalysisState{PitfallStateFinding, PitfallStateSuppressed}
	want["WhileInnerBreak"] = []PitfallAnalysisState{PitfallStateSuppressed, PitfallStateFinding}
	nameAtLine := map[int]string{}
	current := ""
	for index, line := range strings.Split(string(source), "\n") {
		if strings.HasPrefix(line, "fn ") {
			current = strings.SplitN(strings.TrimPrefix(line, "fn "), "(", 2)[0]
		}
		nameAtLine[index+1] = current
	}
	var original map[string][]PitfallAnalysisState
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		analyzer, errors := analyzeSourceWithAnalyzerAtDepth(t, string(source), depth)
		assertPitfallBoundsErrorCount(t, errors, 8)
		got := map[string][]PitfallAnalysisState{}
		for _, result := range analyzer.PitfallAnalysis().Results() {
			if result.Rule != PitfallInclusiveLengthIndex {
				continue
			}
			name := nameAtLine[result.Subject.Source.Line]
			got[name] = append(got[name], result.State)
			if result.State == PitfallStateSuppressed && (result.Suppression == nil || len(result.EvidenceAgainst) == 0) {
				t.Fatalf("suppression lost proof: %+v", result)
			}
			if len(result.Actions) != 1 || result.Actions[0].Kind != PitfallSuggestedEdit {
				t.Fatalf("heuristic range edit changed class: %+v", result)
			}
		}
		incomplete := false
		for _, evaluation := range analyzer.PitfallAnalysis().Evaluations() {
			if evaluation.Rule == PitfallInclusiveLengthIndex {
				incomplete = evaluation.Incomplete
			}
		}
		if !incomplete {
			t.Fatal("uncertain live length was presented as complete proof")
		}
		for name, states := range want {
			if !reflect.DeepEqual(got[name], states) {
				t.Errorf("%s %s = %v, want %v", depth, name, got[name], states)
			}
		}
		if original == nil {
			original = got
		}
		if !reflect.DeepEqual(original, got) {
			t.Fatal("depth changed endpoint semantics")
		}
	}
}

// A Sema-proven never-executed branch retains its owning reachability error,
// without inventing an endpoint finding for the dead index.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability";
// rules/control-flow/flowcontrol_if.md — §20.
func TestPitfallEndpointFlowOmitsSemaProvenDeadAccess(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_endpoint_flow_unreachable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerAtDepth(t, string(source), AnalysisDeep)
	if len(errors) == 0 {
		t.Fatal("invalid fixture did not diagnose unreachable branch")
	}
	found := false
	for _, diagnostic := range errors {
		found = found || diagnostic.ID == diagnostics.UnreachableStatement
	}
	if !found {
		t.Fatalf("missing owning reachability error: %v", errors)
	}
	for _, finding := range analyzer.PitfallAnalysis().Results() {
		if finding.Rule == PitfallInclusiveLengthIndex {
			t.Fatalf("dead access produced endpoint result: %+v", finding)
		}
	}
}
