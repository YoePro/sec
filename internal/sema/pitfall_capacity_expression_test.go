package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Short-circuit proofs protect only the guarded RHS and exact collection/index
// pair. Wrong boundaries, effects, unknown predicates and independent execution
// must not fabricate protection or retroactively protect a left-hand access.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Suppressing evidence", "False-positive control";
// rules/control-flow/flowcontrol_while.md — §5.
func TestPitfallCapacityExpressions(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_expressions.sec")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]PitfallAnalysisState{"CaseLateCheck": PitfallStateFinding, "CaseEqualLateCheck": PitfallStateFinding, "CaseEqualInclusive": PitfallStateFinding}
	for _, name := range []string{"And", "Or", "Reverse", "Negated", "NestedOuter", "NestedLeft", "OrChain", "Inclusive", "EqualAnd", "EqualOr", "EqualReverse", "EqualNegated", "EqualNestedOuter", "EqualNestedLeft", "EqualOrChain"} {
		states["Case"+name] = PitfallStateSuppressed
	}
	for _, name := range []string{"BoundaryAnd", "BoundaryOr", "WrongCollection", "WrongIndex", "MutateCollection", "MutateIndex", "UnknownFlag", "UnrelatedExpr", "Closure", "Deferred", "EqualWrongCollection", "EqualCrossCollection", "EqualWrongBranch", "EqualMutation", "EqualUnrelated", "EqualExpired", "EqualReestablished", "EqualMutateIndex", "EqualClosure", "EqualDeferred"} {
		states["Case"+name] = ""
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "capacity_expressions.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			seen, declared := map[string]int{}, map[string]bool{}
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
				if finding.Classification != PitfallLikelyMistake {
					t.Fatal("advisory became mandatory", finding)
				}
				if finding.State == PitfallStateSuppressed && (finding.Suppression == nil || len(finding.EvidenceAgainst) == 0) {
					t.Fatal("suppression lacks proof", finding)
				}
				if strings.HasPrefix(owner, "CaseEqual") && finding.State == PitfallStateSuppressed && !strings.Contains(finding.Suppression.Reason, "Len == Capacity") {
					t.Fatal("equality mislabeled as strict index guard", finding)
				}
				if owner == "CaseEqualInclusive" && (len(finding.Actions) != 1 || finding.Actions[0].Replacement != "" || !strings.Contains(finding.Actions[0].Title, "strict <")) {
					t.Fatal("inclusive endpoint offers incomplete fix", finding)
				}
				seen[owner]++
			}
			for owner, state := range states {
				want := 1
				if state == "" {
					want = 0
				}
				if !declared[owner] || seen[owner] != want {
					t.Errorf("%s: declared=%v results=%d want=%d", owner, declared[owner], seen[owner], want)
				}
			}
		})
	}
}

// Fresh ASTs and changed strict/equality guards must preserve exact cold/warm evidence
// and invalidate only the edited semantic body.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "Determinism".
func TestPitfallCapacityExpressionCache(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_expressions.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		var cache PitfallCache
		scope := PitfallCacheScope{"capacityexpressions", "plan64", "expression-proofs"}
		checkPitfallCacheRun(t, a, &cache, scope, string(source))
		warm := checkPitfallCacheRun(t, a, &cache, scope, string(source))
		if warm.Hits == 0 {
			t.Fatal("no warm reuse", warm)
		}
		for _, edit := range [][2]string{{"i < values.Len", "i <=values.Len"}, {"values.Len == values.Capacity", "values.Len != values.Capacity"}} {
			checkPitfallCacheRun(t, a, &cache, scope, string(source))
			warm = checkPitfallCacheRun(t, a, &cache, scope, string(source))
			edited := strings.Replace(string(source), edit[0], edit[1], 1)
			if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); usage.Misses != 1 || usage.Hits != warm.Hits-1 {
				t.Fatal("guard edit invalidation", usage, warm)
			}
		}
	}
}
