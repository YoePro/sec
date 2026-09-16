package sema

import (
	"fmt"
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

func TestPitfallAnalysisRecognizesOrderedEndpointExitGuards(t *testing.T) {
	for _, test := range []struct {
		name       string
		condition  string
		suppressed bool
	}{
		{name: "greater or equal", condition: "i >= values.Len", suppressed: true},
		{name: "reversed less or equal", condition: "values.Len <= i", suppressed: true},
		{name: "strict greater misses equality", condition: "i > values.Len"},
		{name: "reversed strict less misses equality", condition: "values.Len < i"},
		{name: "wrong collection", condition: "i >= other.Len"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fmt.Sprintf(`module main
fn Visit(values: ref int[], other: ref int[]) void {
    for i in uint(0)..values.Len {
        if %s {
            break
        }
        let current := values[i]
    }
}
`, test.condition)
			analyzer, errors := analyzeSourceWithAnalyzer(t, source)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			results := analyzer.PitfallAnalysis().Results()
			if len(results) != 1 || results[0].Rule != PitfallInclusiveLengthIndex {
				t.Fatalf("results = %+v, want one inclusive-length result", results)
			}
			if test.suppressed {
				if results[0].State != PitfallStateSuppressed || results[0].Suppression == nil {
					t.Fatalf("safe endpoint exit was not recognized: %+v", results[0])
				}
			} else if results[0].State != PitfallStateFinding {
				t.Fatalf("ineffective endpoint guard hid finding: %+v", results[0])
			}
		})
	}
}

func TestPitfallAnalysisRecognizesElseEndpointExitGuards(t *testing.T) {
	for _, test := range []struct {
		name       string
		condition  string
		exit       string
		suppressed bool
	}{
		{name: "less than", condition: "i < values.Len", exit: "break", suppressed: true},
		{name: "reversed greater than", condition: "values.Len > i", exit: "return", suppressed: true},
		{name: "not equal", condition: "i != values.Len", exit: "break", suppressed: true},
		{name: "less or equal misses endpoint", condition: "i <= values.Len", exit: "break"},
		{name: "wrong collection", condition: "i < other.Len", exit: "break"},
		{name: "conditional exit", condition: "i < values.Len", exit: "if stop { break }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fmt.Sprintf(`module main
fn Visit(values: ref int[], other: ref int[], stop: bool) void {
    for i in uint(0)..values.Len {
        if %s {
        } else {
            %s
        }
        let current := values[i]
    }
}
`, test.condition, test.exit)
			analyzer, errors := analyzeSourceWithAnalyzer(t, source)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			results := analyzer.PitfallAnalysis().Results()
			if len(results) != 1 || results[0].Rule != PitfallInclusiveLengthIndex {
				t.Fatalf("results = %+v, want one inclusive-length result", results)
			}
			if test.suppressed {
				if results[0].State != PitfallStateSuppressed {
					t.Fatalf("safe else exit was not recognized: %+v", results[0])
				}
			} else if results[0].State != PitfallStateFinding {
				t.Fatalf("ineffective else exit hid finding: %+v", results[0])
			}
		})
	}
}

func TestPitfallAnalysisEndpointGuardDoesNotProtectItsOwnIndex(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Visit(values: ref int[]) void {
    for i in uint(0)..values.Len {
        if i >= values.Len {
            let atEnd := values[i]
            break
        }
        let current := values[i]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	results := analyzer.PitfallAnalysis().Results()
	if len(results) != 2 || results[0].State != PitfallStateFinding || results[1].State != PitfallStateSuppressed {
		t.Fatalf("guard-local index must remain a finding and later index be suppressed: %+v", results)
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
