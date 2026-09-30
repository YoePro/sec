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
	if rules[0].ID != PitfallInclusiveLengthIndex || rules[1].ID != PitfallDirectIndexAtLength || rules[2].ID != PitfallBooleanLiteralComparison || rules[3].ID != PitfallExplicitSelfMethodArgument || rules[4].ID != PitfallIneffectiveLengthGuard || rules[5].ID != PitfallUpperNeighborIndex || rules[6].ID != PitfallLowerNeighborIndex || rules[7].ID != PitfallFinalElementNeedsNonEmpty || rules[8].ID != PitfallSkippedFirstElement {
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

func TestPitfallAnalysisFindsExplicitSelfPassedToInstanceMethod(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

type Node struct {
}

impl Node {
    fn Relate(other: Node) void {
    }

    fn Invalid() void {
        self.Relate(self)
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("analysis errors: %v", errors)
	}
	findings := analyzer.PitfallAnalysis().Findings()
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one explicit-self finding", findings)
	}
	finding := findings[0]
	if finding.Rule != PitfallExplicitSelfMethodArgument || finding.Family != PitfallAPIUsage || finding.Classification != PitfallLikelyMistake || finding.Confidence != PitfallConfidenceHigh {
		t.Fatalf("finding = %+v", finding)
	}
	if finding.OwningRule != "method-receiver-and-argument-semantics" || len(finding.EvidenceFor) != 3 {
		t.Fatalf("finding = %+v, want receiver and argument evidence", finding)
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

// Final-element arithmetic is safe only when the same collection is known to
// be non-empty. The canonical preceding empty-check suppresses the finding;
// guards for another collection and non-exiting checks do not.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Final-element access requires non-empty proof"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
//   - rules/analysis/pitfall_analysis.md — "Required bounds and range tests"
func TestPitfallAnalysisRequiresNonEmptyProofForFinalElement(t *testing.T) {
	tests := []struct {
		name       string
		guard      string
		access     string
		wantState  PitfallAnalysisState
		wantResult bool
	}{
		{name: "missing proof", access: "values[values.Len - 1]", wantState: PitfallStateFinding, wantResult: true},
		{name: "exiting empty guard", guard: "if values.Len == 0 { return 0 }", access: "values[values.Len - 1]", wantState: PitfallStateSuppressed, wantResult: true},
		{name: "reversed empty guard", guard: "if 0 == values.Len { return 0 }", access: "values[values.Len - 1]", wantState: PitfallStateSuppressed, wantResult: true},
		{name: "IsEmpty exit guard", guard: "if values.IsEmpty { return 0 }", access: "values[values.Len - 1]", wantState: PitfallStateSuppressed, wantResult: true},
		{name: "wrong collection guard", guard: "if other.Len == 0 { return 0 }", access: "values[values.Len - 1]", wantState: PitfallStateFinding, wantResult: true},
		{name: "wrong IsEmpty collection", guard: "if other.IsEmpty { return 0 }", access: "values[values.Len - 1]", wantState: PitfallStateFinding, wantResult: true},
		{name: "non exiting check", guard: "if values.Len == 0 { let empty := true }", access: "values[values.Len - 1]", wantState: PitfallStateFinding, wantResult: true},
		{name: "non exiting IsEmpty check", guard: "if values.IsEmpty { let empty := true }", access: "values[values.Len - 1]", wantState: PitfallStateFinding, wantResult: true},
		{name: "different indexed collection", access: "other[values.Len - 1]"},
		{name: "different offset", access: "values[values.Len - 2]"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer, errors := analyzeSourceWithAnalyzer(t, fmt.Sprintf(`module main
fn Last(values: ref int[], other: ref int[]) int {
    %s
    return %s
}
`, test.guard, test.access))
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			var final *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallFinalElementNeedsNonEmpty {
					candidate := result
					final = &candidate
					break
				}
			}
			if !test.wantResult {
				if final != nil {
					t.Fatalf("near miss produced final-element result: %+v", *final)
				}
				return
			}
			if final == nil || final.State != test.wantState {
				t.Fatalf("final-element result = %+v, want state %s", final, test.wantState)
			}
			if final.Classification != PitfallLikelyMistake || final.Confidence != PitfallConfidenceHigh || final.OwningRule != "checked-arithmetic-and-bounds" {
				t.Fatalf("incomplete final-element result: %+v", *final)
			}
			if test.wantState == PitfallStateSuppressed && (final.Suppression == nil || len(final.EvidenceAgainst) != 1) {
				t.Fatalf("suppressed result lacks proof evidence: %+v", *final)
			}
		})
	}

	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Last(values: ref int[]) int {
    if values.Len == 0 {
        return 0
    } else {
        return values[values.Len - 1]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("else-branch analysis errors: %v", errors)
	}
	results := analyzer.PitfallAnalysis().Results()
	if len(results) != 1 || results[0].Rule != PitfallFinalElementNeedsNonEmpty || results[0].State != PitfallStateSuppressed {
		t.Fatalf("equality false branch did not retain non-empty proof: %+v", results)
	}

	analyzer, errors = analyzeSourceWithAnalyzer(t, `module main
fn Last(values: ref int[]) int {
    if values.IsEmpty {
        return 0
    } else {
        return values[values.Len - 1]
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("IsEmpty else-branch analysis errors: %v", errors)
	}
	results = analyzer.PitfallAnalysis().Results()
	if len(results) != 1 || results[0].Rule != PitfallFinalElementNeedsNonEmpty || results[0].State != PitfallStateSuppressed {
		t.Fatalf("IsEmpty false branch did not retain non-empty proof: %+v", results)
	}
}

// Direct Len-versus-zero conditions establish a non-empty fact only in the
// corresponding branch. The fact intentionally expires after that branch's
// first straight-line statement so a structural mutation cannot leave stale
// suppression evidence.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Final-element access requires non-empty proof"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func TestPitfallAnalysisConsumesBranchLocalNonEmptyProof(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		body      string
		wantState PitfallAnalysisState
	}{
		{name: "greater than zero", condition: "values.Len > 0", body: "return values[values.Len - 1]", wantState: PitfallStateSuppressed},
		{name: "reversed greater than zero", condition: "0 < values.Len", body: "return values[values.Len - 1]", wantState: PitfallStateSuppressed},
		{name: "not equal zero", condition: "values.Len != 0", body: "return values[values.Len - 1]", wantState: PitfallStateSuppressed},
		{name: "negated IsEmpty", condition: "!values.IsEmpty", body: "return values[values.Len - 1]", wantState: PitfallStateSuppressed},
		{name: "empty branch remains unsafe", condition: "values.IsEmpty", body: "return values[values.Len - 1]", wantState: PitfallStateFinding},
		{name: "wrong collection", condition: "other.Len > 0", body: "return values[values.Len - 1]", wantState: PitfallStateFinding},
		{name: "proof expires after statement", condition: "values.Len > 0", body: "let observed := values.Len\n        return values[values.Len - 1]", wantState: PitfallStateFinding},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer, errors := analyzeSourceWithAnalyzer(t, fmt.Sprintf(`module main
fn Last(values: ref int[], other: ref int[]) int {
    if %s {
        %s
    }
    return 0
}
`, test.condition, test.body))
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			var final *PitfallFinding
			for _, result := range analyzer.PitfallAnalysis().Results() {
				if result.Rule == PitfallFinalElementNeedsNonEmpty {
					candidate := result
					final = &candidate
					break
				}
			}
			if final == nil || final.State != test.wantState {
				t.Fatalf("final-element result = %+v, want state %s", final, test.wantState)
			}
		})
	}
}

// Inclusive acceptance and strict-greater rejection both leave index == Len
// reachable. The analysis correlates resolved binding and collection identity,
// while strict guards and mismatched subjects remain negative cases.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Ineffective upper bounds guard"
//   - rules/analysis/pitfall_analysis.md — "Ineffective rejection guard"
//   - rules/analysis/pitfall_analysis.md — "Required control-flow tests"
func TestPitfallAnalysisFindsIneffectiveLengthGuards(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		access    string
		rejection bool
		finding   bool
	}{
		{name: "inclusive upper guard", condition: "index <= values.Len", access: "values[index]", finding: true},
		{name: "reversed inclusive upper guard", condition: "values.Len >= index", access: "values[index]", finding: true},
		{name: "strict upper guard", condition: "index < values.Len", access: "values[index]"},
		{name: "wrong upper collection", condition: "index <= other.Len", access: "values[index]"},
		{name: "wrong upper binding", condition: "otherIndex <= values.Len", access: "values[index]"},
		{name: "strict greater rejection", condition: "index > values.Len", access: "values[index]", rejection: true, finding: true},
		{name: "reversed strict rejection", condition: "values.Len < index", access: "values[index]", rejection: true, finding: true},
		{name: "safe rejection", condition: "index >= values.Len", access: "values[index]", rejection: true},
		{name: "wrong rejection collection", condition: "index > other.Len", access: "values[index]", rejection: true},
		{name: "wrong rejection binding", condition: "otherIndex > values.Len", access: "values[index]", rejection: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := fmt.Sprintf("if %s { return %s }\n    return 0", test.condition, test.access)
			if test.rejection {
				body = fmt.Sprintf("if %s { return 0 }\n    return %s", test.condition, test.access)
			}
			analyzer, errors := analyzeSourceWithAnalyzer(t, fmt.Sprintf(`module main
fn Access(values: ref int[], other: ref int[], index: uint, otherIndex: uint) int {
    %s
}
`, body))
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			findings := analyzer.PitfallAnalysis().Findings()
			if !test.finding {
				if len(findings) != 0 {
					t.Fatalf("safe or unrelated guard produced findings: %+v", findings)
				}
				return
			}
			if len(findings) != 1 || findings[0].Rule != PitfallIneffectiveLengthGuard {
				t.Fatalf("findings = %+v, want one ineffective-length-guard finding", findings)
			}
			finding := findings[0]
			if finding.Classification != PitfallProvenInvalid || finding.Confidence != PitfallConfidenceProven || finding.OwningRule != "bounds" || len(finding.EvidenceFor) != 2 || len(finding.Actions) != 1 {
				t.Fatalf("incomplete structured finding: %+v", finding)
			}
		})
	}
}

// Neighbor checks use the normalized zero-based half-open domain and resolved
// identities. Starting predecessor traversal at one, shortening the upper
// domain, guarding the access, or indexing another collection are safe
// counterexamples for this initial high-confidence slice.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Upper neighbor access"
//   - rules/analysis/pitfall_analysis.md — "Lower neighbor access"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func TestPitfallAnalysisFindsUnsafeNeighborIndexes(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
		body  string
		rule  PitfallRuleID
	}{
		{name: "upper neighbor", start: "uint(0)", end: "values.Len", body: "let neighbor := values[i + 1]", rule: PitfallUpperNeighborIndex},
		{name: "reversed upper neighbor", start: "uint(0)", end: "values.Len", body: "let neighbor := values[1 + i]", rule: PitfallUpperNeighborIndex},
		{name: "lower neighbor", start: "uint(0)", end: "values.Len", body: "let neighbor := values[i - 1]", rule: PitfallLowerNeighborIndex},
		{name: "shortened upper domain", start: "uint(0)", end: "values.Len - 1", body: "let neighbor := values[i + 1]"},
		{name: "predecessor starts at one", start: "uint(1)", end: "values.Len", body: "let neighbor := values[i - 1]"},
		{name: "different collection", start: "uint(0)", end: "values.Len", body: "let neighbor := other[i + 1]"},
		{name: "different binding", start: "uint(0)", end: "values.Len", body: "let neighbor := values[otherIndex + 1]"},
		{name: "guarded upper neighbor", start: "uint(0)", end: "values.Len", body: "if i + 1 < values.Len { let neighbor := values[i + 1] }"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer, errors := analyzeSourceWithAnalyzer(t, fmt.Sprintf(`module main
fn Visit(values: ref int[], other: ref int[], otherIndex: uint) void {
    for i in %s..<%s {
        %s
    }
}
`, test.start, test.end, test.body))
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			findings := analyzer.PitfallAnalysis().Findings()
			if test.rule == "" {
				if len(findings) != 0 {
					t.Fatalf("safe or unrelated neighbor traversal produced findings: %+v", findings)
				}
				return
			}
			if len(findings) != 1 || findings[0].Rule != test.rule {
				t.Fatalf("findings = %+v, want one %s finding", findings, test.rule)
			}
			finding := findings[0]
			if finding.Classification != PitfallProvenInvalid || finding.Confidence != PitfallConfidenceProven || finding.OwningRule != "bounds" || len(finding.EvidenceFor) != 2 || len(finding.Actions) != 1 {
				t.Fatalf("incomplete neighbor finding: %+v", finding)
			}
		})
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

// A proven endpoint exit suppresses the inclusive-length rule. The assertion
// follows the canonical rule registry instead of freezing its current size so
// unrelated catalog additions cannot invalidate this suppression test.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Suppressing evidence"
//   - rules/analysis/pitfall_analysis.md — "Inclusive upper bound against collection length"
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
	if len(evaluations) != len(PitfallRules()) {
		t.Fatalf("evaluations = %+v", evaluations)
	}
	for _, evaluation := range evaluations {
		if evaluation.Rule != PitfallInclusiveLengthIndex {
			continue
		}
		if evaluation.State != PitfallStateSuppressed || evaluation.FindingCount != 0 || evaluation.SuppressedCount != 1 {
			t.Fatalf("inclusive-length evaluation = %+v", evaluation)
		}
		return
	}
	t.Fatalf("missing %s evaluation in %+v", PitfallInclusiveLengthIndex, evaluations)
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
			inclusive, found := pitfallResultForRule(results, PitfallInclusiveLengthIndex)
			if !found {
				t.Fatalf("results = %+v, want one inclusive-length result", results)
			}
			if test.suppressed {
				if inclusive.State != PitfallStateSuppressed || inclusive.Suppression == nil {
					t.Fatalf("safe endpoint exit was not recognized: %+v", inclusive)
				}
			} else if inclusive.State != PitfallStateFinding {
				t.Fatalf("ineffective endpoint guard hid finding: %+v", inclusive)
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

// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall
// reasoning", "Inclusive upper bound against collection length";
// rules/control-flow/flowcontrol_while.md — §14 "continue".
func TestPitfallAnalysisRecognizesEndpointContinueGuards(t *testing.T) {
	for _, test := range []struct {
		name       string
		guard      string
		suppressed bool
	}{
		{name: "equal endpoint", guard: "if i == values.Len { continue }", suppressed: true},
		{name: "ordered endpoint", guard: "if i >= values.Len { continue }", suppressed: true},
		{name: "else endpoint", guard: "if i < values.Len {} else { continue }", suppressed: true},
		{name: "strict greater misses endpoint", guard: "if i > values.Len { continue }"},
		{name: "conditional continue", guard: "if i == values.Len { if stop { continue } }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fmt.Sprintf(`module main
fn Visit(values: ref int[], stop: bool) void {
    for i in uint(0)..values.Len {
        %s
        let current := values[i]
    }
}
`, test.guard)
			analyzer, errors := analyzeSourceWithAnalyzer(t, source)
			if len(errors) != 0 {
				t.Fatalf("analysis errors: %v", errors)
			}
			results := analyzer.PitfallAnalysis().Results()
			inclusive, found := pitfallResultForRule(results, PitfallInclusiveLengthIndex)
			if !found {
				t.Fatalf("results = %+v, want one inclusive-length result", results)
			}
			want := PitfallStateFinding
			if test.suppressed {
				want = PitfallStateSuppressed
			}
			if inclusive.State != want {
				t.Fatalf("guard result = %+v, want %s", inclusive, want)
			}
		})
	}
}

func pitfallResultForRule(results []PitfallFinding, rule PitfallRuleID) (PitfallFinding, bool) {
	for _, result := range results {
		if result.Rule == rule {
			return result, true
		}
	}
	return PitfallFinding{}, false
}

// Rules: rules/analysis/pitfall_analysis.md — "Reachability" and "Guards
// participate in pitfall reasoning".
func TestPitfallAnalysisContinueGuardDoesNotProtectItsOwnIndex(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
fn Visit(values: ref int[]) void {
    for i in uint(0)..values.Len {
        if i == values.Len {
            let atEnd := values[i]
            continue
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
