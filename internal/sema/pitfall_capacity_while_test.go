package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Canonical while capacity bounds produce only advisory findings. Valid live
// length/equality proofs suppress them at the access, while unknown, stale,
// different and separately executed index relationships produce no guess.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Suppressing evidence", "False-positive control", "Required test structure".
func TestPitfallCapacityWhile(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_while.sec")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]PitfallAnalysisState{}
	for _, name := range []string{"Strict", "Reverse", "Negated", "Inclusive", "StaleEquality"} {
		states["Case"+name] = PitfallStateFinding
	}
	for _, name := range []string{"Equal", "ConditionEqual", "InnerEqual", "ConditionGuard", "InnerGuard", "ExitGuard", "AssertGuard", "BothBounds", "FreshEquality", "ShortCircuit"} {
		states["Case"+name] = PitfallStateSuppressed
	}
	for _, name := range []string{"Other", "Or", "IndexChanged", "Deferred", "Closure", "DifferentIndex", "NoIndex", "Callee"} {
		states["Case"+name] = ""
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "capacity_while.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			seen := map[string]int{}
			declared := map[string]bool{}
			for _, statement := range program.Statements {
				if fn, ok := statement.(*ast.FunctionDeclaration); ok {
					declared[fn.Name.Value] = true
				}
			}
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
				if finding.Classification != PitfallLikelyMistake || len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit {
					t.Fatal("heuristic gained mandatory or automatic-fix authority", finding)
				}
				if finding.State == PitfallStateSuppressed && finding.Suppression == nil {
					t.Fatal("suppression lacks evidence", finding)
				}
				if owner == "CaseInclusive" && (!strings.Contains(finding.Actions[0].Title, "strict <") || finding.Actions[0].Replacement != "") {
					t.Fatal("inclusive-bound advice must also require a strict comparison", finding)
				}
				seen[owner]++
			}
			for owner := range states {
				if !declared[owner] {
					t.Errorf("missing fixture case %s", owner)
				}
				want := 1
				if states[owner] == "" {
					want = 0
				}
				if seen[owner] != want {
					t.Errorf("%s: got %d results, want %d", owner, seen[owner], want)
				}
			}
		})
	}
}

// Exact warm/cold results include the loop's per-access guard witnesses.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "Determinism".
func TestPitfallCapacityWhileCache(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_while.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		var cache PitfallCache
		scope := PitfallCacheScope{"capacitywhile", "plan64", "capacity-while"}
		checkPitfallCacheRun(t, a, &cache, scope, string(source))
		if usage := checkPitfallCacheRun(t, a, &cache, scope, string(source)); usage.Hits == 0 {
			t.Fatal("no warm body reuse", usage)
		}
	}
}
