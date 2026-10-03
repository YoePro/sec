package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeIntervalSource(t *testing.T, depth AnalysisDepth, body string) *Analyzer {
	t.Helper()
	source := `module main

type Reading struct {
	value: int,
}

impl Reading {
	property Level: int {
		get {
			return self.value
		}
	}
}

fn Check(value: int, count: uint, reading: Reading, ratio: float64) bool {
	` + body + `
}
`
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	analyzer := NewAnalyzerWithDepth(depth)
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	return analyzer
}

func intervalResult(analyzer *Analyzer, rule PitfallRuleID) *PitfallFinding {
	for _, result := range analyzer.PitfallAnalysis().Results() {
		if result.Rule == rule {
			candidate := result
			return &candidate
		}
	}
	return nil
}

// Integer interval conditions over one resolved value and literal bounds are
// proven tautological with `||` when the half-lines cover every integer and
// proven impossible with `&&` when they do not meet. An inclusive tautology
// suggests canonical range membership rather than an operator swap; mirrored
// operand order is normalized, and near misses stay silent.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Tautological interval conditions"
//   - rules/analysis/pitfall_analysis.md — "Required control-flow tests"
func TestPitfallIntervalConditionsAreProvenTautologicalOrImpossible(t *testing.T) {
	tests := []struct {
		name        string
		condition   string
		rule        PitfallRuleID
		replacement string
	}{
		{name: "inclusive tautology suggests membership", condition: "value >= 0 || value <= 10", rule: PitfallTautologicalInterval, replacement: "value in 0..10"},
		{name: "mirrored tautology", condition: "0 <= value || 10 >= value", rule: PitfallTautologicalInterval, replacement: "value in 0..10"},
		{name: "adjacent strict tautology", condition: "value > 4 || value < 6", rule: PitfallTautologicalInterval},
		{name: "unsigned tautology", condition: "count >= 3 || count <= 2", rule: PitfallTautologicalInterval},
		{name: "impossible interval", condition: "value > 10 && value < 5", rule: PitfallImpossibleInterval},
		{name: "strict bounds leave no integer", condition: "value > 4 && value < 5", rule: PitfallImpossibleInterval},
		{name: "negative bounds", condition: "value >= -1 && value <= -5", rule: PitfallImpossibleInterval},
		{name: "outside test is ordinary", condition: "value < 0 || value > 10"},
		{name: "different subjects", condition: "value >= 0 || count <= 10"},
		{name: "float subject is not totally ordered", condition: "ratio >= 0.0 || ratio <= 10.0"},
		{name: "strict gap is not a tautology", condition: "value > 5 || value < 5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer := analyzeIntervalSource(t, AnalysisStandard, "return "+test.condition)
			for _, rule := range []PitfallRuleID{PitfallTautologicalInterval, PitfallImpossibleInterval} {
				result := intervalResult(analyzer, rule)
				if rule != test.rule {
					if result != nil {
						t.Fatalf("unexpected %s result: %+v", rule, *result)
					}
					continue
				}
				if result == nil || result.State != PitfallStateFinding || result.Classification != PitfallLikelyMistake ||
					result.Confidence != PitfallConfidenceProven || len(result.EvidenceFor) != 4 {
					t.Fatalf("%s result = %+v", rule, result)
				}
				if test.replacement == "" {
					if len(result.Actions) != 0 {
						t.Fatalf("actions = %+v, want none", result.Actions)
					}
					continue
				}
				if len(result.Actions) != 1 || result.Actions[0].Replacement != test.replacement || result.Actions[0].Kind != PitfallSuggestedEdit {
					t.Fatalf("actions = %+v, want %q", result.Actions, test.replacement)
				}
			}
		})
	}
}

// A non-empty literal interval written as two comparisons is exactly
// canonical range membership, offered as a proven Deep-analysis fix. A
// property subject may run a getter, so evaluating it once instead of twice
// is not proven equivalent and the rewrite is suppressed.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Canonical idiom guidance"
//   - rules/analysis/pitfall_analysis.md — "Required control-flow tests"
func TestPitfallRangeMembershipIdiomRequiresPureSubject(t *testing.T) {
	tests := []struct {
		name        string
		condition   string
		state       PitfallAnalysisState
		replacement string
	}{
		{name: "inclusive interval", condition: "value >= 0 && value <= 10", state: PitfallStateFinding, replacement: "value in 0..10"},
		{name: "half-open interval", condition: "value >= 0 && value < 10", state: PitfallStateFinding, replacement: "value in 0..<10"},
		{name: "stored field", condition: "reading.value >= 1 && reading.value <= 9", state: PitfallStateFinding, replacement: "reading.value in 1..9"},
		{name: "property getter", condition: "reading.Level >= 1 && reading.Level <= 9", state: PitfallStateSuppressed},
		{name: "strict lower bound has no direct spelling", condition: "value > 0 && value <= 10"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer := analyzeIntervalSource(t, AnalysisDeep, "return "+test.condition)
			result := intervalResult(analyzer, PitfallRangeMembershipIdiom)
			if test.state == "" {
				if result != nil {
					t.Fatalf("unexpected idiom result: %+v", *result)
				}
				return
			}
			if result == nil || result.State != test.state || result.Classification != PitfallSuspiciousIntent {
				t.Fatalf("idiom result = %+v, want state %s", result, test.state)
			}
			if test.state == PitfallStateSuppressed {
				if result.Suppression == nil || len(result.EvidenceAgainst) != 1 || len(result.Actions) != 0 {
					t.Fatalf("suppressed idiom lacks evidence or kept a rewrite: %+v", *result)
				}
				return
			}
			if len(result.Actions) != 1 || result.Actions[0].Kind != PitfallProvenFix || result.Actions[0].Replacement != test.replacement {
				t.Fatalf("actions = %+v, want %q", result.Actions, test.replacement)
			}
		})
	}

	// The idiom is optional insight: Standard analysis does not evaluate it.
	analyzer := analyzeIntervalSource(t, AnalysisStandard, "return value >= 0 && value <= 10")
	if result := intervalResult(analyzer, PitfallRangeMembershipIdiom); result != nil {
		t.Fatalf("Standard analysis produced idiom result: %+v", *result)
	}
}

// The suggested membership is valid Sec with the same meaning.
func TestPitfallRangeMembershipReplacementIsValidSec(t *testing.T) {
	analyzeIntervalSource(t, AnalysisStandard, "return value in 0..10 || value in 0..<10")
}
