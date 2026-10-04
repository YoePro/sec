package sema

import "testing"

// Capacity is not the live length: a traversal to Capacity that indexes the
// collection is a likely mistake unless Len == Capacity is proven on the path
// and the loop leaves the collection's structure unchanged.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Capacity used as live length"
func TestPitfallAnalysisFindsCapacityUsedAsLiveLength(t *testing.T) {
	tests := []struct {
		name      string
		before    string
		wrap      bool
		body      string
		wantState PitfallAnalysisState
		want      bool
	}{
		{name: "indexed capacity traversal", body: "Consume(values[i])", wantState: PitfallStateFinding, want: true},
		{name: "asserted equality", before: "assert values.Len == values.Capacity", body: "Consume(values[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "reversed asserted equality", before: "assert values.Capacity == values.Len", body: "Consume(values[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "enclosing if equality", wrap: true, body: "Consume(values[i])", wantState: PitfallStateSuppressed, want: true},
		{name: "mutation after the proof", before: "assert values.Len == values.Capacity\n    values.Clear()", body: "Consume(values[i])", wantState: PitfallStateFinding, want: true},
		{name: "mutation inside the loop", before: "assert values.Len == values.Capacity", body: "Consume(values[i])\n        discard values.RemoveAt(0)", wantState: PitfallStateFinding, want: true},
		{name: "other collection indexed", body: "Consume(other[i])"},
		{name: "capacity without indexing", body: "Consume(1)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loop := "for i in uint(0)..<values.Capacity {\n        " + test.body + "\n    }"
			if test.wrap {
				loop = "if values.Len == values.Capacity {\n        for i in uint(0)..<values.Capacity {\n            " + test.body + "\n        }\n    }"
			}
			analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Consume(value: int) void {}
fn Walk(values: ref mut list[int], other: ref list[int]) void {
    `+test.before+`
    `+loop+`
}
`)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			var found *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallCapacityAsLength {
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
				t.Fatal("missing capacity-as-length result")
			}
			if found.State != test.wantState {
				t.Fatalf("state = %s, want %s: %+v", found.State, test.wantState, *found)
			}
			if found.State == PitfallStateFinding && (found.Classification != PitfallLikelyMistake || len(found.Actions) != 1 || found.Actions[0].Replacement != "Len") {
				t.Fatalf("finding = %+v", *found)
			}
		})
	}
}

// A direct index counted from Capacity is never live at Capacity itself and
// live below it only under an unproven Len == Capacity assumption.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Capacity used as live length"
func TestPitfallAnalysisFindsDirectCapacityIndex(t *testing.T) {
	tests := []struct {
		name               string
		before             string
		access             string
		wantState          PitfallAnalysisState
		wantClassification PitfallClassification
		want               bool
	}{
		{name: "index at capacity", access: "values[values.Capacity]", wantState: PitfallStateFinding, wantClassification: PitfallProvenInvalid, want: true},
		{name: "index at capacity despite equality", before: "assert values.Len == values.Capacity", access: "values[values.Capacity]", wantState: PitfallStateFinding, wantClassification: PitfallProvenInvalid, want: true},
		{name: "last by capacity", access: "values[values.Capacity - 1]", wantState: PitfallStateFinding, wantClassification: PitfallLikelyMistake, want: true},
		{name: "asserted equality", before: "assert values.Capacity == values.Len", access: "values[values.Capacity - 2]", wantState: PitfallStateSuppressed, wantClassification: PitfallLikelyMistake, want: true},
		{name: "mutation after the proof", before: "assert values.Len == values.Capacity\n    values.Clear()", access: "values[values.Capacity - 1]", wantState: PitfallStateFinding, wantClassification: PitfallLikelyMistake, want: true},
		{name: "other collection", access: "other[values.Capacity - 1]"},
		{name: "zero offset is not counted back", access: "values[values.Capacity - 0]"},
		{name: "length based", access: "values[values.Len - 1]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Consume(value: int) void {}
fn Walk(values: ref mut list[int], other: ref list[int]) void {
    `+test.before+`
    Consume(`+test.access+`)
}
`)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			var found *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallCapacityAsLength {
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
				t.Fatal("missing capacity-as-length result")
			}
			if found.State != test.wantState || found.Classification != test.wantClassification {
				t.Fatalf("state = %s, classification = %s, want %s, %s: %+v", found.State, found.Classification, test.wantState, test.wantClassification, *found)
			}
		})
	}
}
