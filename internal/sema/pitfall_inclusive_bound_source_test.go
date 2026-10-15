package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Inclusive endpoints require strictly greater indexed length. Equality and
// non-strict comparisons, stale or mutated relations cannot suppress advice.
// Explicit positive constant steps share this domain; runtime steps do not.
// Rules: rules/control-flow/flowcontrol_for.md — explicit step and §29.
// Rules: rules/analysis/pitfall_analysis.md — "Bound source differs from indexed collection",
// "Suppressing evidence", "Guards participate in pitfall reasoning".
func TestPitfallInclusiveBoundSource(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_inclusive_bound_source.sec")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]PitfallAnalysisState{}
	states["CaseUnknown"] = PitfallStateFinding
	states["CaseEqual"] = PitfallStateFinding
	states["CaseNonStrict"] = PitfallStateFinding
	states["CaseStrict"] = PitfallStateSuppressed
	states["CaseReverse"] = PitfallStateSuppressed
	states["CaseExit"] = PitfallStateSuppressed
	states["CaseEnclosing"] = PitfallStateSuppressed
	states["CaseWrongRelation"] = PitfallStateFinding
	states["CaseMutation"] = PitfallStateFinding
	states["CaseCallee"] = PitfallStateFinding
	states["CaseBodyMutation"] = PitfallStateFinding
	states["CaseBodyCallee"] = PitfallStateFinding
	states["CaseStaleCondition"] = PitfallStateFinding
	states["CaseSame"] = ""
	states["CaseParallel"] = ""
	states["CaseUnusedBound"] = ""
	states["CaseOffset"] = ""
	states["CaseStep"] = PitfallStateFinding
	states["CaseStepStrict"] = PitfallStateSuppressed
	states["CaseStepEqual"] = PitfallStateFinding
	states["CaseStepNonStrict"] = PitfallStateFinding
	states["CaseStepMutation"] = PitfallStateFinding
	states["CaseStepRuntime"] = ""
	states["CaseStepFolded"] = PitfallStateSuppressed
	states["CaseStepExit"] = PitfallStateSuppressed
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "inclusive_bound.sec"))
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
				if finding.Rule != PitfallWrongBoundSource {
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
				if len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Replacement != "" || !strings.Contains(finding.Actions[0].Title, "half-open") {
					t.Fatal("bound-source advice must stay a suggested edit", finding)
				}
				if strings.HasPrefix(owner, "CaseStep") {
					stepProof := false
					for _, evidence := range finding.EvidenceFor {
						if strings.Contains(evidence.Fact, "positive integer constant") && evidence.Strength == PitfallEvidenceProof && evidence.Source.Line > 0 {
							stepProof = true
						}
					}
					if !stepProof {
						t.Fatal("missing canonical step witness", finding)
					}
				}
				seen[owner]++
			}
			for name, state := range states {
				want := 1
				if state == "" {
					want = 0
				}
				if seen[name] != want {
					t.Errorf("%s: got %d findings, want %d", name, seen[name], want)
				}
			}
		})
	}
}

// Endpoint and step proof edits must invalidate one additional body and retain exact warm
// findings and navigable evidence. Rules: rules/analysis/pitfall_analysis.md —
// "Incremental analysis", "Determinism".
func TestPitfallInclusiveBoundSourceCache(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_inclusive_bound_source.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		var cache PitfallCache
		scope := PitfallCacheScope{"inclusivebound", "plan64", "inclusive-proof"}
		checkPitfallCacheRun(t, a, &cache, scope, string(source))
		warm := checkPitfallCacheRun(t, a, &cache, scope, string(source))
		for _, edit := range [][2]string{
			{"assert right.Len > left.Len", "assert right.Len >=left.Len"},
			{"step delta", "step 2u   "},
		} {
			checkPitfallCacheRun(t, a, &cache, scope, string(source))
			warm = checkPitfallCacheRun(t, a, &cache, scope, string(source))
			edited := strings.Replace(string(source), edit[0], edit[1], 1)
			if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); warm.Hits == 0 || usage.Misses != warm.Misses+1 || usage.Hits != warm.Hits-1 || usage.Unavailable != warm.Unavailable {
				t.Fatal("endpoint or step proof edit invalidation", usage, warm)
			}
		}
	}
}
