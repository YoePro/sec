package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeIntervalSource(t *testing.T, depth AnalysisDepth, body string) *Analyzer {
	t.Helper()
	source := `module main

let FLOOR := 0
let LIMIT := 10

type Percent int range 0..100

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

fn Check(value: int, count: uint, reading: Reading, ratio: float64, letter: char, scalar: rune, percent: Percent, flag: bool) bool {
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
				assertIntervalConstantFix(t, *result)
				intentActions := result.Actions[:len(result.Actions)-1]
				if test.replacement == "" {
					if len(intentActions) != 0 {
						t.Fatalf("actions = %+v, want none", result.Actions)
					}
					continue
				}
				if len(intentActions) != 1 || intentActions[0].Replacement != test.replacement || intentActions[0].Kind != PitfallSuggestedEdit {
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

func intervalResults(analyzer *Analyzer, rule PitfallRuleID) []PitfallFinding {
	results := []PitfallFinding{}
	for _, result := range analyzer.PitfallAnalysis().Results() {
		if result.Rule == rule {
			results = append(results, result)
		}
	}
	return results
}

// Interval proofs extend beyond one literal pair: bounds may be named
// compile-time constants resolved in their own scope, subjects may be char
// values ordered by their one byte or rune values ordered by Unicode scalar
// (a non-ASCII character literal names no single char byte), chains longer than one pair are
// decided by their tightest (`&&`) or loosest (`||`) bounds with unrelated
// operands ignored. Each chain is reported once.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Tautological interval conditions"
//   - rules/foundations/operators.md — "Character ordering", "Rune ordering"
func TestPitfallIntervalConditionsUseConstantsCharsAndChains(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		rule        PitfallRuleID
		replacement string
		evidence    int
	}{
		{name: "module constant bound", body: "return value >= LIMIT && value < 5", rule: PitfallImpossibleInterval, evidence: 5},
		{name: "local constant bound", body: "let low := 20\n\treturn value > low && value <= LIMIT", rule: PitfallImpossibleInterval, evidence: 6},
		{name: "constant tautology keeps the written names", body: "return value >= FLOOR || value <= LIMIT", rule: PitfallTautologicalInterval, replacement: "value in FLOOR..LIMIT", evidence: 6},
		{name: "mutable binding is not a bound", body: "let mut low := 20\n\tlow += 0\n\treturn value > low && value < 5"},
		{name: "sibling scopes resolve their own constant", body: "if flag {\n\t\tlet bound := 20\n\t\treturn value > 3 && value < bound\n\t}\n\tlet bound := 1\n\treturn value > 3 && value < bound", rule: PitfallImpossibleInterval, evidence: 5},
		{name: "char impossible interval", body: "return letter >= 'z' && letter <= 'a'", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "char tautology suggests char range", body: "return letter >= 'a' || letter <= 'z'", rule: PitfallTautologicalInterval, replacement: "letter in 'a'..'z'", evidence: 4},
		{name: "rune impossible interval", body: "return scalar > 'z' && scalar < 'a'", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "char outside test is ordinary", body: "return letter < 'a' || letter > 'z'"},
		{name: "char byte bounds", body: "return letter > 200t && letter < 100t", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "Latin-1 char literal is its code point (MD-043)", body: "return letter >= 'é' && letter < 'a'", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "Latin-1 char interval is ordinary", body: "return letter >= 'a' && letter <= 'é'"},
		{name: "non-ASCII rune literal is a scalar", body: "return scalar >= 'é' && scalar < 'a'", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "conjunction chain with unrelated operand", body: "return value > 10 && flag && value < 5", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "conjunction chain uses tightest bounds", body: "return value > 4 && value < 20 && value < 3", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "disjunction chain has no whole-condition rewrite", body: "return value >= 0 || flag || value <= 10", rule: PitfallTautologicalInterval, evidence: 4},
		{name: "disjunction chain uses loosest bounds", body: "return value > 50 || value <= 0 || value >= 1", rule: PitfallTautologicalInterval, evidence: 4},
		{name: "satisfiable chain is ordinary", body: "return value >= 0 && value <= 10 && value < 5"},
		{name: "chain of other subjects is ordinary", body: "return value > 10 && count < 5 && flag"},
		{name: "constrained subject uses the integer proof", body: "return percent > 60 && percent < 40", rule: PitfallImpossibleInterval, evidence: 4},
		{name: "constrained interior stays ordinary", body: "return percent < 10 || percent > 90"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer := analyzeIntervalSource(t, AnalysisDeep, test.body)
			for _, rule := range []PitfallRuleID{PitfallTautologicalInterval, PitfallImpossibleInterval} {
				results := intervalResults(analyzer, rule)
				if rule != test.rule {
					if len(results) != 0 {
						t.Fatalf("unexpected %s results: %+v", rule, results)
					}
					continue
				}
				if len(results) != 1 {
					t.Fatalf("%s results = %+v, want exactly one", rule, results)
				}
				result := results[0]
				if result.State != PitfallStateFinding || result.Confidence != PitfallConfidenceProven || len(result.EvidenceFor) != test.evidence {
					t.Fatalf("%s result = %+v, want %d evidence items", rule, result, test.evidence)
				}
				assertIntervalConstantFix(t, result)
				intentActions := result.Actions[:len(result.Actions)-1]
				if test.replacement == "" && len(intentActions) != 0 {
					t.Fatalf("actions = %+v, want none", result.Actions)
				}
				if test.replacement != "" && (len(intentActions) != 1 || intentActions[0].Replacement != test.replacement) {
					t.Fatalf("actions = %+v, want %q", result.Actions, test.replacement)
				}
			}
		})
	}
}

// Named constants and char literals yield canonical membership with the
// written bound spelling, and the suggested forms are valid Sec.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Canonical idiom guidance"
func TestPitfallRangeMembershipIdiomWithNamedAndCharBounds(t *testing.T) {
	tests := []struct {
		condition   string
		replacement string
	}{
		{condition: "value >= FLOOR && value <= LIMIT", replacement: "value in FLOOR..LIMIT"},
		{condition: "value >= FLOOR && value < LIMIT", replacement: "value in FLOOR..<LIMIT"},
		{condition: "letter >= 'a' && letter <= 'z'", replacement: "letter in 'a'..'z'"},
		{condition: "'0' <= letter && '9' >= letter", replacement: "letter in '0'..'9'"},
	}
	for _, test := range tests {
		t.Run(test.condition, func(t *testing.T) {
			analyzer := analyzeIntervalSource(t, AnalysisDeep, "return "+test.condition)
			result := intervalResult(analyzer, PitfallRangeMembershipIdiom)
			if result == nil || result.State != PitfallStateFinding || len(result.Actions) != 1 ||
				result.Actions[0].Kind != PitfallProvenFix || result.Actions[0].Replacement != test.replacement {
				t.Fatalf("idiom result = %+v, want %q", result, test.replacement)
			}
			analyzeIntervalSource(t, AnalysisStandard, "return "+test.replacement)
		})
	}

	// A longer chain is not one interval, so no whole-condition rewrite is offered.
	analyzer := analyzeIntervalSource(t, AnalysisDeep, "return value >= 0 && value <= 10 && flag")
	if result := intervalResult(analyzer, PitfallRangeMembershipIdiom); result != nil {
		t.Fatalf("chain produced idiom result: %+v", *result)
	}
}

// A constrained type domain cannot decide an interval chain that the integer
// proof leaves open, because Sema rejects every constant bound outside the
// subject's domain: the only bounds that could make the domain matter.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Meaningless comparisons from proven ranges"
//   - rules/types/contracts.md — range contracts
func TestPitfallIntervalDomainBoundsAreRejectedBySema(t *testing.T) {
	for _, condition := range []string{
		"percent > 100 && percent < 150",
		"percent <= 100 || percent >= 150",
		"percent > -1 && percent < -5",
	} {
		source := "module main\n\ntype Percent int range 0..100\n\nfn Check(percent: Percent) bool {\n\treturn " + condition + "\n}\n"
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		if errors := NewAnalyzerWithDepth(AnalysisDeep).Analyze(program); len(errors) == 0 {
			t.Fatalf("%s: out-of-domain bound was accepted; interval proofs must then consult the type domain", condition)
		}
	}
}

// assertIntervalConstantFix distinguishes the equivalent constant simplification
// from the independently suggested intent repair in the existing interval matrix.
// Rules: rules/analysis/pitfall_analysis.md — "Corrective actions", "Fix safety".
func assertIntervalConstantFix(t *testing.T, result PitfallFinding) {
	t.Helper()
	if len(result.Actions) == 0 {
		t.Fatal("missing constant action", result)
	}
	action := result.Actions[len(result.Actions)-1]
	want := "false"
	if result.Rule == PitfallTautologicalInterval {
		want = "true"
	}
	if action.Kind != PitfallProvenFix || action.Replacement != want || !action.Safety.verifiedFor(result.Rule, want) {
		t.Fatal(action)
	}
}
