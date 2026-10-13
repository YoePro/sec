package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestPitfallNonEmptyFlow consumes contracts and dominating proofs while
// retaining mutation, wrong-subject and independent-execution controls.
// Rules: rules/analysis/pitfall_analysis.md — Final-element access requires
// non-empty proof, Guards participate in pitfall reasoning, False-positive control.
func TestPitfallNonEmptyFlow(t *testing.T) {
	path := "../../testdata/sema/pitfall_nonempty_flow.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]PitfallAnalysisState{}
	for _, name := range []string{"Fixed", "Present", "Minimum", "Exact", "Inherited", "DominatingExit", "Asserted", "Nested", "Conjunction", "ElseExit", "RelationalExit"} {
		states["Case"+name] = PitfallStateSuppressed
	}
	for _, name := range []string{"Maximum", "ZeroMinimum", "OtherCollection", "Mutated", "CalleeMutation", "LoopMutation", "Deferred"} {
		states["Case"+name] = PitfallStateFinding
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		seen := map[string]bool{}
		for _, finding := range a.PitfallAnalysis().Results() {
			if finding.Rule != PitfallFinalElementNeedsNonEmpty {
				continue
			}
			owner := ""
			for _, statement := range program.Statements {
				if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Token.Line <= finding.Subject.Source.Line {
					owner = fn.Name.Value
				}
			}
			if finding.State != states[owner] {
				t.Errorf("%s/%s: got %s want %s", depth, owner, finding.State, states[owner])
			}
			if finding.State == PitfallStateSuppressed && (finding.Suppression == nil || len(finding.EvidenceAgainst) == 0) {
				t.Fatal(finding)
			}
			seen[owner] = true
		}
		for name := range states {
			if !seen[name] {
				t.Errorf("missing %s/%s", depth, name)
			}
		}
	}
}
