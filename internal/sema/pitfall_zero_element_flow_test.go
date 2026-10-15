package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Zero handling survives reads and element writes, but never structural changes,
// replacement, mutable calls, conditional RHS or independently executed bodies.
// Rules: rules/analysis/pitfall_analysis.md — "Skipped-first-element advisory",
// "Suppressing evidence", "False-positive control".
func TestPitfallZeroElementFlow(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_zero_element_flow.sec")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]PitfallAnalysisState{}
	states["CasePlain"] = PitfallStateSuppressed
	states["CaseRead"] = PitfallStateSuppressed
	states["CaseElementWrite"] = PitfallStateSuppressed
	states["CaseMutation"] = PitfallStateFinding
	states["CaseCallee"] = PitfallStateFinding
	states["CaseConditionalMutation"] = PitfallStateFinding
	states["CaseFresh"] = PitfallStateSuppressed
	states["CaseOtherMutation"] = PitfallStateFinding
	states["CaseConditionalZero"] = PitfallStateFinding
	states["CaseAnd"] = PitfallStateFinding
	states["CaseOr"] = PitfallStateFinding
	states["CaseLeft"] = PitfallStateSuppressed
	states["CaseNegatedLeft"] = PitfallStateSuppressed
	states["CaseOther"] = PitfallStateFinding
	states["CaseDeferred"] = PitfallStateFinding
	states["CaseClosure"] = PitfallStateFinding
	states["CaseSameExpressionMutation"] = PitfallStateFinding
	states["CaseReplacement"] = PitfallStateFinding
	states["CaseReplacementFresh"] = PitfallStateSuppressed
	states["CaseBranchBoth"] = PitfallStateSuppressed
	states["CaseBranchMissing"] = PitfallStateFinding
	states["CaseBranchExitTrue"] = PitfallStateSuppressed
	states["CaseBranchExitFalse"] = PitfallStateSuppressed
	states["CaseBranchNested"] = PitfallStateSuppressed
	states["CaseBranchMutation"] = PitfallStateFinding
	states["CaseBranchFresh"] = PitfallStateSuppressed
	states["CaseBranchExitingMutation"] = PitfallStateSuppressed
	states["CaseBranchLaterMutation"] = PitfallStateFinding
	states["CaseBranchConstantTrue"] = PitfallStateSuppressed
	states["CaseBranchDeferred"] = PitfallStateFinding
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "zero_element.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			seen := map[string]int{}
			for _, finding := range a.PitfallAnalysis().Results() {
				if finding.Rule != PitfallSkippedFirstElement {
					continue
				}
				owner := ""
				for _, statement := range program.Statements {
					if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Token.Line <= finding.Subject.Source.Line {
						owner = fn.Name.Value
					}
				}
				if finding.State != states[owner] {
					t.Errorf("%s: got %s want %s: %+v", owner, finding.State, states[owner], finding)
				}
				if finding.Classification != PitfallLikelyMistake {
					t.Errorf("heuristic became a validity error: %+v", finding)
				}
				if finding.State == PitfallStateSuppressed && (finding.Suppression == nil || len(finding.EvidenceAgainst) == 0) {
					t.Fatal("suppression lacks evidence", finding)
				}
				if len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Replacement != "" || !strings.Contains(finding.Actions[0].Title, "index 0") {
					t.Fatal("bound-source advice must stay a suggested edit", finding)
				}
				if finding.State == PitfallStateSuppressed && strings.HasPrefix(owner, "CaseBranch") {
					counts := map[string]int{"CaseBranchBoth": 2, "CaseBranchExitTrue": 1, "CaseBranchExitFalse": 1, "CaseBranchNested": 3, "CaseBranchFresh": 2, "CaseBranchExitingMutation": 1, "CaseBranchConstantTrue": 1}
					if len(finding.EvidenceAgainst) != counts[owner] || len(finding.Suppression.Evidence) != counts[owner] {
						t.Fatal("branch witnesses lost", owner, finding)
					}
					for _, evidence := range finding.EvidenceAgainst {
						if evidence.Source.Line == 0 || evidence.Strength != PitfallEvidenceSuppressing {
							t.Fatal("missing branch proof location", finding)
						}
					}
				}
				seen[owner]++
			}
			for name, state := range states {
				want := 1
				if state == "" || depth == AnalysisInteractive {
					want = 0
				}
				if seen[name] != want {
					t.Errorf("%s: got %d findings, want %d", name, seen[name], want)
				}
			}
		})
	}
}

// Mutation edits revoke old zero handling and invalidate only the edited body.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "Determinism".
func TestPitfallZeroElementFlowCache(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_zero_element_flow.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		var cache PitfallCache
		scope := PitfallCacheScope{"zeroelementflow", "plan64", "zero-intent"}
		checkPitfallCacheRun(t, a, &cache, scope, string(source))
		warm := checkPitfallCacheRun(t, a, &cache, scope, string(source))
		for _, edit := range [][2]string{
			{"discard values.Len", "values.Clear()"},
			{"else { discard values[0] }", "else { discard other[0] }"},
		} {
			checkPitfallCacheRun(t, a, &cache, scope, string(source))
			warm = checkPitfallCacheRun(t, a, &cache, scope, string(source))
			replacement := edit[1] + strings.Repeat(" ", len(edit[0])-len(edit[1]))
			edited := strings.Replace(string(source), edit[0], replacement, 1)
			if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); warm.Hits == 0 || usage.Misses != warm.Misses+1 || usage.Hits != warm.Hits-1 || usage.Unavailable != warm.Unavailable {
				t.Fatal("mutation or branch witness edit invalidation", usage, warm)
			}
		}
	}
}
