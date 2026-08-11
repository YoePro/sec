package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

func TestPitfallRuleRegistryIsStableAndDefensive(t *testing.T) {
	rules := PitfallRules()
	if len(rules) < 2 {
		t.Fatalf("rules = %v, want initial bounds registry", rules)
	}
	if rules[0].ID != PitfallInclusiveLengthIndex || rules[1].ID != PitfallDirectIndexAtLength || rules[2].ID != PitfallBooleanLiteralComparison {
		t.Fatalf("unexpected rule order: %v", rules)
	}
	if rules[0].MinimumDepth != AnalysisInteractive || rules[0].DefaultConfidence != PitfallConfidenceProven {
		t.Fatalf("inclusive rule metadata = %+v", rules[0])
	}
	rules[0].RequiredFacts[0] = "mutated"
	if PitfallRules()[0].RequiredFacts[0] == "mutated" {
		t.Fatal("PitfallRules returned mutable registry storage")
	}
}

func TestPitfallAnalysisFindsDirectIndexAtLength(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

fn Direct(values: ref int[]) int {
    return values[values.Len]
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	findings := analyzer.PitfallAnalysis().Findings()
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one", findings)
	}
	finding := findings[0]
	if finding.Rule != PitfallDirectIndexAtLength || finding.Classification != PitfallProvenInvalid || finding.Confidence != PitfallConfidenceProven {
		t.Fatalf("finding = %+v", finding)
	}
	if finding.OwningRule != "bounds" || len(finding.EvidenceFor) != 2 || len(finding.Actions) != 1 {
		t.Fatalf("incomplete structured finding: %+v", finding)
	}
}

func TestPitfallAnalysisFindsInclusiveLengthTraversal(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

fn Visit(values: ref int[]) void {
    for i in uint(0)..values.Len {
        let current := values[i]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	findings := analyzer.PitfallAnalysis().Findings()
	if len(findings) != 1 || findings[0].Rule != PitfallInclusiveLengthIndex {
		t.Fatalf("findings = %+v, want inclusive-length finding", findings)
	}
	if findings[0].Actions[0].Replacement != "..<" {
		t.Fatalf("action = %+v, want half-open suggestion", findings[0].Actions[0])
	}
}

func TestPitfallAnalysisUsesSemanticCollectionAndBindingIdentity(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

fn Visit(left: ref int[], right: ref int[]) void {
    for i in uint(0)..<left.Len {
        let leftValue := left[i]
    }
    for i in uint(0)..left.Len {
        let rightValue := right[i]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	if findings := analyzer.PitfallAnalysis().Findings(); len(findings) != 0 {
		t.Fatalf("unrelated collection or half-open range produced findings: %+v", findings)
	}
}

func TestPitfallAnalysisSuppressesGuardedInclusiveEndpoint(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

fn Visit(values: ref int[]) void {
    for i in uint(0)..values.Len {
        if i == values.Len {
            break
        }
        let current := values[i]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	analysis := analyzer.PitfallAnalysis()
	if findings := analysis.Findings(); len(findings) != 0 {
		t.Fatalf("guarded endpoint produced findings: %+v", findings)
	}
	results := analysis.Results()
	if len(results) != 1 || results[0].State != PitfallStateSuppressed || results[0].Suppression == nil {
		t.Fatalf("suppressed result = %+v", results)
	}
	evaluations := analysis.Evaluations()
	if len(evaluations) != 3 || evaluations[0].State != PitfallStateSuppressed || evaluations[0].SuppressedCount != 1 {
		t.Fatalf("evaluations = %+v", evaluations)
	}
}

func TestPitfallAnalysisSimplifiesOneBitRegisterTruthComparison(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

type Status register[1] {
    DataReady: bit[1]
}

impl Status {
    fn Ready() bool {
        return bool(self.DataReady == 1)
    }
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.OperatorNonComparable {
		t.Fatalf("errors = %+v, want one bool/int comparison error", errors)
	}
	if !strings.Contains(errors[0].Help, "Did you mean `self.DataReady`?") {
		t.Fatalf("diagnostic help = %q, want intended direct bool access", errors[0].Help)
	}

	findings := analyzer.PitfallAnalysis().Findings()
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one coalescible boolean finding", findings)
	}
	finding := findings[0]
	if finding.Rule != PitfallBooleanLiteralComparison || finding.Classification != PitfallProvenInvalid || finding.Confidence != PitfallConfidenceProven {
		t.Fatalf("finding = %+v", finding)
	}
	if len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Replacement != "self.DataReady" {
		t.Fatalf("action = %+v, want direct field access", finding.Actions)
	}
	if !strings.HasPrefix(finding.Subject.Expression, "bool(") {
		t.Fatalf("subject = %q, want outer redundant conversion coalesced", finding.Subject.Expression)
	}
}

func TestPitfallAnalysisSimplifiesFalseAndReversedTruthComparisons(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		replacement string
	}{
		{name: "equals zero", expression: "value == 0", replacement: "!value"},
		{name: "not equals one", expression: "value != 1", replacement: "!value"},
		{name: "reversed one", expression: "1 == value", replacement: "value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer, errors := analyzeSourceWithAnalyzer(t, "module main\nfn Check(value: bool) bool { return bool("+test.expression+") }\n")
			if len(errors) != 1 || !strings.Contains(errors[0].Help, "`"+test.replacement+"`") {
				t.Fatalf("errors = %+v, want suggestion %q", errors, test.replacement)
			}
			findings := analyzer.PitfallAnalysis().Findings()
			if len(findings) != 1 || findings[0].Actions[0].Replacement != test.replacement {
				t.Fatalf("findings = %+v, want replacement %q", findings, test.replacement)
			}
		})
	}
}

func TestPitfallAnalysisProvesRedundantBooleanLiteralComparison(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Check(value: bool) bool {
    return bool(value == true)
}
`)
	if len(errors) != 0 {
		t.Fatalf("errors = %+v", errors)
	}
	findings := analyzer.PitfallAnalysis().Findings()
	if len(findings) != 1 || findings[0].Classification != PitfallLikelyMistake {
		t.Fatalf("findings = %+v", findings)
	}
	if findings[0].Actions[0].Kind != PitfallProvenFix || findings[0].Actions[0].Replacement != "value" {
		t.Fatalf("action = %+v, want proven direct-value fix", findings[0].Actions)
	}
}

func TestPitfallAnalysisDoesNotInferTruthFromOtherIntegers(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Check(value: bool) bool {
    return bool(value == 2)
}
`)
	if len(errors) != 1 || strings.Contains(errors[0].Help, "Did you mean") {
		t.Fatalf("errors = %+v, want ordinary incompatible-type diagnostic", errors)
	}
	if findings := analyzer.PitfallAnalysis().Findings(); len(findings) != 0 {
		t.Fatalf("findings = %+v, must not infer boolean intent from 2", findings)
	}
}

func TestPitfallAnalysisDoesNotSuppressConditionalEndpointBreak(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

fn Visit(values: ref int[], stop: bool) void {
    for i in uint(0)..values.Len {
        if i == values.Len {
            if stop {
                break
            }
        }
        let current := values[i]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	findings := analyzer.PitfallAnalysis().Findings()
	if len(findings) != 1 || findings[0].Rule != PitfallInclusiveLengthIndex {
		t.Fatalf("conditional break unsoundly suppressed endpoint finding: %+v", findings)
	}
}

func TestPitfallAnalysisSnapshotIsDefensive(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

fn Direct(values: ref int[]) int {
    return values[values.Len]
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	first := analyzer.PitfallAnalysis()
	first.results[0].EvidenceFor[0].Fact = "mutated"
	first.evaluations[0].FindingCount = 99
	again := analyzer.PitfallAnalysis()
	if again.Results()[0].EvidenceFor[0].Fact == "mutated" || again.Evaluations()[0].FindingCount == 99 {
		t.Fatal("PitfallAnalysis returned mutable analyzer storage")
	}
}
