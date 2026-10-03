package sema

import "testing"

// Structural mutation inside an indexed traversal is classified rather than
// reported everywhere: proven invalid, likely mistake, or a proven safe
// structural pattern that is retained as a suppressed result.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Structural mutation during indexed iteration"
func TestPitfallAnalysisClassifiesStructuralMutationDuringIndexedIteration(t *testing.T) {
	tests := []struct {
		name               string
		loop               string
		body               string
		wantState          PitfallAnalysisState
		wantClassification PitfallClassification
		want               bool
	}{
		{name: "remove at loop index", body: "if values[i] < 0 {\n            discard values.RemoveAt(i)\n        }", wantState: PitfallStateFinding, wantClassification: PitfallLikelyMistake, want: true},
		{name: "insert at loop index", body: "if values[i] < 0 {\n            discard try values.Insert(i, 0)\n        }", wantState: PitfallStateFinding, wantClassification: PitfallLikelyMistake, want: true},
		{name: "clear then index", body: "values.Clear()\n        Consume(values[i])", wantState: PitfallStateFinding, wantClassification: PitfallProvenInvalid, want: true},
		{name: "remove then break", body: "if values[i] < 0 {\n            discard values.RemoveAt(i)\n            break\n        }", wantState: PitfallStateSuppressed, want: true},
		{name: "remove then return", body: "if values[i] < 0 {\n            discard values.RemoveAt(i)\n            return Ok()\n        }", wantState: PitfallStateSuppressed, want: true},
		{name: "break inside nested loop does not end traversal", body: "while flag {\n            discard values.RemoveAt(i)\n            break\n        }", wantState: PitfallStateFinding, wantClassification: PitfallLikelyMistake, want: true},
		{name: "append beyond range", body: "Consume(values[i])\n        discard try values.Append(7)", wantState: PitfallStateSuppressed, want: true},
		{name: "draining loop", loop: "_", body: "discard values.RemoveAt(0)", wantState: PitfallStateSuppressed, want: true},
		{name: "other collection mutated", body: "Consume(values[i])\n        discard other.RemoveAt(i)"},
		{name: "no structural mutation", body: "values[i] = 0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding := "i"
			if test.loop != "" {
				binding = test.loop
			}
			analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Consume(value: int) void {}
fn Walk(values: ref mut list[int], other: ref mut list[int], flag: bool) Result[void, CollectionError] {
    for `+binding+` in uint(0)..<values.Len {
        `+test.body+`
    }
    return Ok()
}
`)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			var found *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallIndexedStructuralMutation {
					candidate := result
					found = &candidate
					break
				}
			}
			if !test.want {
				if found != nil {
					t.Fatalf("unexpected result: %+v", *found)
				}
				return
			}
			if found == nil {
				t.Fatal("missing structural-mutation result")
			}
			if found.State != test.wantState {
				t.Fatalf("state = %s, want %s: %+v", found.State, test.wantState, *found)
			}
			if test.wantState == PitfallStateFinding && found.Classification != test.wantClassification {
				t.Fatalf("classification = %s, want %s", found.Classification, test.wantClassification)
			}
			if test.wantState == PitfallStateSuppressed && (found.Suppression == nil || len(found.EvidenceAgainst) == 0) {
				t.Fatalf("suppressed result lacks evidence: %+v", *found)
			}
		})
	}
}
