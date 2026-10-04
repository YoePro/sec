package sema

import "testing"

// A comparison whose result the operand's type range already decides is
// optional Deep information; comparisons the type range leaves open, constant
// comparisons, and shallower depths produce no finding.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Meaningless comparisons from proven ranges"
func TestPitfallAnalysisFindsMeaninglessComparisons(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		want      bool
		always    string
	}{
		{name: "unsigned length below zero", condition: "values.Len < 0", want: true, always: "false"},
		{name: "unsigned at least zero", condition: "count >= 0", want: true, always: "true"},
		{name: "mirrored literal", condition: "0 > count", want: true, always: "false"},
		{name: "byte above its maximum", condition: "small > 255", want: true, always: "false"},
		{name: "byte at most its maximum", condition: "small <= 255", want: true, always: "true"},
		{name: "contract at most its maximum", condition: "percent <= 100", want: true, always: "true"},
		{name: "contract range", condition: "percent > 100", want: true, always: "false"},
		{name: "contract range at bound", condition: "percent >= 100"},
		{name: "signed below zero", condition: "signed < 0"},
		{name: "byte inside its range", condition: "small > 200"},
		{name: "constant comparison", condition: "limit < 0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := `module main
type Percent int range 0..100
fn Check(values: ref list[int], count: uint, small: uint8, percent: Percent, signed: int) bool {
    let limit := 5
    return ` + test.condition + `
}
`
			analyzer, errors := analyzeSourceWithAnalyzerAtDepth(t, source, AnalysisDeep)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			var found *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallMeaninglessComparison {
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
				t.Fatal("missing meaningless-comparison result")
			}
			if found.Classification != PitfallSuspiciousIntent || found.Confidence != PitfallConfidenceProven || len(found.EvidenceFor) != 2 ||
				found.EvidenceFor[1].Fact != "so comparing it with "+literalOf(test.condition)+" is always "+test.always {
				t.Fatalf("finding = %+v", *found)
			}
			shallow, _ := analyzeSourceWithAnalyzerAtDepth(t, source, AnalysisStandard)
			for _, result := range shallow.PitfallAnalysis().Results() {
				if result.Rule == PitfallMeaninglessComparison {
					t.Fatalf("Standard depth evaluated a Deep rule: %+v", result)
				}
			}
		})
	}
}

func literalOf(condition string) string {
	for _, literal := range []string{"255", "100", "0"} {
		if len(condition) >= len(literal) && (condition[len(condition)-len(literal):] == literal || condition[:len(literal)] == literal) {
			return literal
		}
	}
	return ""
}
