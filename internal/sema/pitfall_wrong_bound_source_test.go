package sema

import "testing"

// A loop bounded by one collection's Len that indexes only another collection
// is a likely copy/paste mistake unless the indexed collection is proven at
// least as long; a parallel traversal that also uses the bounding collection
// is left to the owning bounds rule.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Bound source differs from indexed collection", "Parallel collection traversal"
//   - rules/analysis/pitfall_analysis.md — "Required collection-relation tests"
func TestPitfallAnalysisFindsWrongBoundSource(t *testing.T) {
	tests := []struct {
		name      string
		before    string
		wrap      string
		body      string
		wantState PitfallAnalysisState
		want      bool
	}{
		{name: "wrong collection supplies the bound", body: "Consume(right[i])", wantState: PitfallStateFinding, want: true},
		{name: "asserted equal length", before: "assert left.Len == right.Len", body: "Consume(right[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "asserted sufficient length", before: "assert right.Len >= left.Len", body: "Consume(right[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "insufficient assertion", before: "assert right.Len <= left.Len", body: "Consume(right[i])", wantState: PitfallStateFinding, want: true},
		{name: "exit guard on unequal lengths", before: "if left.Len != right.Len {\n        return\n    }", body: "Consume(right[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "exit guard on shorter indexed collection", before: "if right.Len < left.Len {\n        return\n    }", body: "Consume(right[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "enclosing if proof", wrap: "right.Len >= left.Len", body: "Consume(right[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "mutation after the proof", before: "assert left.Len == right.Len\n    right.Clear()", body: "Consume(right[i])", wantState: PitfallStateFinding, want: true},
		{name: "parallel traversal", body: "Consume(left[i] + right[i])"},
		{name: "same collection", body: "Consume(left[i])"},
		{name: "bound collection otherwise used", body: "Consume(right[i])\n        discard left.Len"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loop := "for i in uint(0)..<left.Len {\n        " + test.body + "\n    }"
			if test.wrap != "" {
				loop = "if " + test.wrap + " {\n        for i in uint(0)..<left.Len {\n            " + test.body + "\n        }\n    }"
			}
			analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Consume(value: int) void {}
fn Walk(left: ref list[int], right: ref mut list[int]) void {
    `+test.before+`
    `+loop+`
}
`)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			var found *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallWrongBoundSource {
					candidate := result
					found = &candidate
				}
			}
			if !test.want {
				if found != nil {
					t.Fatalf("unexpected result: %+v", *found)
				}
				return
			}
			if found == nil {
				t.Fatal("missing wrong-bound-source result")
			}
			if found.State != test.wantState {
				t.Fatalf("state = %s, want %s: %+v", found.State, test.wantState, *found)
			}
			if found.State == PitfallStateFinding && (found.Classification != PitfallLikelyMistake || len(found.Actions) != 1 || found.Actions[0].Replacement != "right.Len") {
				t.Fatalf("finding = %+v", *found)
			}
		})
	}
}
