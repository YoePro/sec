package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func lengthEndpointResult(t *testing.T, body string, rule PitfallRuleID) *PitfallFinding {
	t.Helper()
	analyzer, errors := analyzeSourceWithAnalyzerAtDepth(t, `module main
fn Consume(value: int) void {}
fn Compare(left: int, right: int) void {}
fn Walk(values: ref int[]) void {
    `+body+`
}
`, AnalysisStandard)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	for _, result := range analyzer.PitfallAnalysis().Results() {
		if result.Rule == rule {
			candidate := result
			return &candidate
		}
	}
	return nil
}

// Rules:
//   - rules/analysis/pitfall_analysis.md — "Omitted-last-element advisory"
func TestPitfallAnalysisFindsOmittedLastElement(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantState PitfallAnalysisState
		want      bool
	}{
		{name: "traversal stops before the final element", body: "for i in uint(0)..<values.Len - 1 {\n        Consume(values[i])\n    }", wantState: PitfallStateFinding, want: true},
		{name: "pairwise neighbor traversal", body: "for i in uint(0)..<values.Len - 1 {\n        Compare(values[i], values[i + 1])\n    }", wantState: PitfallStateSuppressed, want: true},
		{name: "final element handled separately", body: "for i in uint(0)..<values.Len - 1 {\n        Consume(values[i])\n    }\n    Consume(values[values.Len - 1])", wantState: PitfallStateSuppressed, want: true},
		{name: "complete traversal", body: "for i in uint(0)..<values.Len {\n        Consume(values[i])\n    }"},
		{name: "no indexing", body: "for i in uint(0)..<values.Len - 1 {\n        Consume(1)\n    }"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found := lengthEndpointResult(t, test.body, PitfallOmittedLastElement)
			if !test.want {
				if found != nil {
					t.Fatalf("unexpected result: %+v", *found)
				}
				return
			}
			if found == nil || found.State != test.wantState {
				t.Fatalf("result = %+v, want state %s", found, test.wantState)
			}
		})
	}
}

// Rules:
//   - rules/analysis/pitfall_analysis.md — "Avoiding fragile `0..len - 1` workarounds"
func TestPitfallAnalysisFindsFragileInclusiveLengthTraversal(t *testing.T) {
	unproven := lengthEndpointResult(t, "for i in uint(0)..values.Len - 1 {\n        Consume(values[i])\n    }", PitfallFragileInclusiveLength)
	if unproven == nil || unproven.Classification != PitfallLikelyMistake || len(unproven.Actions) != 1 ||
		unproven.Actions[0].Kind != PitfallSuggestedEdit || unproven.Actions[0].Replacement != "uint(0)..<values.Len" {
		t.Fatalf("unproven result = %+v", unproven)
	}
	proven := lengthEndpointResult(t, "if values.Len == 0 {\n        return\n    }\n    for i in uint(0)..values.Len - 1 {\n        Consume(values[i])\n    }", PitfallFragileInclusiveLength)
	if proven == nil || proven.Classification != PitfallSuspiciousIntent || proven.Confidence != PitfallConfidenceProven ||
		len(proven.Actions) != 1 || proven.Actions[0].Kind != PitfallProvenFix {
		t.Fatalf("proven result = %+v", proven)
	}
	if found := lengthEndpointResult(t, "for i in uint(0)..<values.Len {\n        Consume(values[i])\n    }", PitfallFragileInclusiveLength); found != nil {
		t.Fatalf("canonical traversal reported: %+v", *found)
	}
}

func analyzeSourceWithAnalyzerAtDepth(t *testing.T, input string, depth AnalysisDepth) (*Analyzer, []Error) {
	t.Helper()
	p := parser.New(lexer.New(ensureModuleForTest(input)))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	analyzer := NewAnalyzerWithDepth(depth)
	return analyzer, analyzer.Analyze(program)
}
