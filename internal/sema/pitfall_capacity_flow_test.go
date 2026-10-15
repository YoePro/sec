package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Capacity equality follows dominating proofs across blocks, but never a
// sibling, stale collection, loop backedge or independently executed body.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Suppressing evidence", "Guards participate in pitfall reasoning".
func TestPitfallCapacityProofFlow(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_flow.sec")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]PitfallAnalysisState{}
	for _, name := range []string{"OuterAssert", "OuterLoop", "ExitGuard", "ElseExit", "FalseBranch", "Negated", "FalseDisjunction", "ElementWrite", "Reestablished", "ConditionReestablished", "ExitConditionReestablished", "DirectAnd", "DirectOr", "DirectReverse", "DirectNegation", "DirectNested", "DirectChain", "DirectReestablished"} {
		states["Case"+name] = PitfallStateSuppressed
	}
	for _, name := range []string{"Mutation", "Callee", "ConditionMutation", "Sibling", "Other", "Deferred", "Closure", "Backedge", "ConditionalMutation", "Replacement", "DirectWrongBranch", "DirectWrongCollection", "DirectCrossCollection", "DirectLateCheck", "DirectMutation", "DirectExpired", "DirectSibling", "DirectClosure", "DirectDeferred"} {
		states["Case"+name] = PitfallStateFinding
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "capacity_flow.sec"))
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
				if finding.Rule != PitfallCapacityAsLength {
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
				if len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit {
					t.Fatal("capacity advice must stay a suggested edit", finding)
				}
				seen[owner]++
			}
			for name := range states {
				if seen[name] != 1 {
					t.Errorf("%s: got %d findings, want one", name, seen[name])
				}
			}
		})
	}
}

// Warm/cold findings must preserve the same proofs and source navigation.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "Determinism".
func TestPitfallCapacityProofCache(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_flow.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		var cache PitfallCache
		scope := PitfallCacheScope{"capacityflow", "plan64", "capacity-proof-flow"}
		checkPitfallCacheRun(t, a, &cache, scope, string(source))
		warm := checkPitfallCacheRun(t, a, &cache, scope, string(source))
		if warm.Hits == 0 {
			t.Fatal("no warm body reuse", warm)
		}
		for _, edit := range [][2]string{
			{"assert values.Len == values.Capacity", "discard values.Len"},
			{"values.Len == values.Capacity && values[", "values.Len != values.Capacity && values["},
		} {
			checkPitfallCacheRun(t, a, &cache, scope, string(source))
			warm = checkPitfallCacheRun(t, a, &cache, scope, string(source))
			old, replacement := edit[0], edit[1]
			edited := strings.Replace(string(source), old, replacement+strings.Repeat(" ", len(old)-len(replacement)), 1)
			if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); usage.Misses != 1 || usage.Hits != warm.Hits-1 {
				t.Fatal("proof edit did not invalidate precisely its body", usage, warm)
			}
		}
	}
}
