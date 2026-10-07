package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestPitfallSourcePriority keeps optional scheduling deterministic and bounded
// while mandatory semantic checking remains independent of admission order.
// Rules: rules/analysis/pitfall_analysis.md — "Interactive analysis", "Analysis states";
// rules/compiler/compiler_analysis.md — §15.
func TestPitfallSourcePriority(t *testing.T) {
	file := "../../testdata/sema/pitfall_lsp_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(data), file)).ParseProgram()
	// Two real parsed bodies share the module but have distinct source owners.
	first := parsed.Statements[1].(*ast.FunctionDeclaration)
	second := parsed.Statements[2].(*ast.FunctionDeclaration)
	second.Token.File = "focused.sec"
	baseline := NewAnalyzerWithDepth(AnalysisInteractive)
	baseline.SetPitfallBudget(10, 64)
	errors := baseline.Analyze(parsed)
	original := baseline.PitfallAnalysis()
	focused := NewAnalyzerWithDepth(AnalysisInteractive)
	focused.SetPitfallBudget(10, 64)
	focused.SetPitfallSourcePriority([]string{"focused.sec"})
	if actual := focused.Analyze(parsed); !reflect.DeepEqual(actual, errors) {
		t.Fatal("scheduling changed validity", actual, errors)
	}
	analysis := focused.PitfallAnalysis()
	has := func(p *PitfallAnalysis, rule PitfallRuleID) bool {
		for _, f := range p.Findings() {
			if f.Rule == rule {
				return true
			}
		}
		return false
	}
	if !has(original, PitfallBooleanLiteralComparison) || has(analysis, PitfallBooleanLiteralComparison) || !has(analysis, PitfallTautologicalInterval) {
		t.Fatal(first, original.Coverage(), analysis.Coverage(), original.Findings(), analysis.Findings())
	}
	if analysis.Coverage().VisitedNodes > 10 || analysis.Coverage().SkippedUnits == 0 {
		t.Fatal(analysis.Coverage())
	}
	previous := analysis.Results()
	focused.Analyze(parsed)
	if !reflect.DeepEqual(previous, focused.PitfallAnalysis().Results()) {
		t.Fatal("priority reuse nondeterministic")
	}
	focused.SetPitfallSourcePriority(nil)
	focused.Analyze(parsed)
	if !reflect.DeepEqual(original.Results(), focused.PitfallAnalysis().Results()) {
		t.Fatal("cleared priority retained scheduling")
	}
}
