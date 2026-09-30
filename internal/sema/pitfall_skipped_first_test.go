package sema

import "testing"

// A canonical start-at-one traversal is advisory only when the loop directly
// indexes the collection. Predecessor traversal and dominating element-zero
// handling are required false-positive suppressions.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Skipped-first-element advisory"
//   - rules/analysis/pitfall_analysis.md — "Required bounds and range tests"
func TestPitfallAnalysisFindsAndSuppressesSkippedFirstElement(t *testing.T) {
	tests := []struct {
		name      string
		before    string
		body      string
		wantState PitfallAnalysisState
		want      bool
	}{
		{name: "direct traversal skips zero", body: "Consume(values[i])", wantState: PitfallStateFinding, want: true},
		{name: "predecessor traversal", body: "Compare(values[i - 1], values[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "zero handled first", before: "Consume(values[0])", body: "Consume(values[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "conditional zero handling does not dominate", before: "if enabled { Consume(values[0]) }", body: "Consume(values[i])", wantState: PitfallStateFinding, want: true},
		{name: "different collection handled first", before: "Consume(other[0])", body: "Consume(values[i])", wantState: PitfallStateFinding, want: true},
		{name: "zero based traversal", body: "Consume(values[i])"},
		{name: "different indexed collection", body: "Consume(other[i])"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start := "uint(1)"
			if test.name == "zero based traversal" {
				start = "uint(0)"
			}
			analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Consume(value: int) void {}
fn Compare(left: int, right: int) void {}
fn Walk(values: ref int[], other: ref int[], enabled: bool) void {
    `+test.before+`
    for i in `+start+`..<values.Len {
        `+test.body+`
    }
}
`)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}

			var skipped *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallSkippedFirstElement {
					candidate := result
					skipped = &candidate
					break
				}
			}
			if !test.want {
				if skipped != nil {
					t.Fatalf("near miss produced skipped-first result: %+v", *skipped)
				}
				return
			}
			if skipped == nil || skipped.State != test.wantState {
				t.Fatalf("skipped-first result = %+v, want state %s", skipped, test.wantState)
			}
			if skipped.Classification != PitfallLikelyMistake || skipped.Confidence != PitfallConfidenceHigh {
				t.Fatalf("incomplete skipped-first result: %+v", *skipped)
			}
			if test.wantState == PitfallStateSuppressed && (skipped.Suppression == nil || len(skipped.EvidenceAgainst) != 1) {
				t.Fatalf("suppressed result lacks semantic evidence: %+v", *skipped)
			}
		})
	}
}
