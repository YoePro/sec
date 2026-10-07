package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestPitfallGuardFlow verifies reachable nested paths, later uses, refreshed
// guards, sibling isolation and mutation invalidation at every analysis depth.
// Rules: rules/analysis/pitfall_analysis.md — "Required control-flow tests",
// "Reachability", "Guards participate in pitfall reasoning".
func TestPitfallGuardFlow(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/pitfall_guard_flow_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{
		"WrongSwitch":                 {1, 0},
		"CheckSwitch":                 {0, 1},
		"WrongMatch":                  {1, 0},
		"CheckMatch":                  {0, 1},
		"ExpiredLength":               {0, 0},
		"CheckChangedLength":          {0, 0},
		"InvalidatedLengthGuard":      {1, 0},
		"LoopChangedLength":           {1, 0},
		"ElementWritePreservesLength": {0, 0},
		"WrongNested":                 {1, 0},
		"WrongLater":                  {1, 0},
		"ProtectedNested":             {0, 0},
		"ProtectedElse":               {0, 0},
		"ProtectedExit":               {0, 0},
		"InvalidatedOuter":            {1, 0},
		"ExpiredWitness":              {0, 0},
		"BranchSibling":               {1, 0},
		"CheckLater":                  {0, 1},
		"CheckNested":                 {0, 1},
		"CheckProtected":              {0, 0},
		"CheckLaterExit":              {0, 0},
		"CheckChanged":                {0, 0},
		"CheckChangedBranch":          {0, 0},
		"CheckMutableCall":            {0, 0},
		"CheckSibling":                {0, 1},
		"WrongWhile":                  {1, 0},
		"WrongFor":                    {1, 0},
		"ProtectedFor":                {0, 0},
		"WrongUnsafe":                 {1, 0},
		"CheckWhile":                  {0, 1},
		"CheckReturn":                 {0, 0},
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), "guard_flow.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		findings := a.PitfallAnalysis().Results()
		for position, statement := range program.Statements {
			function, ok := statement.(*ast.FunctionDeclaration)
			if !ok {
				continue
			}
			expected, selected := want[function.Name.Value]
			if !selected {
				continue
			}
			var got [2]int
			for _, finding := range findings {
				if finding.Subject.Source.Line < function.Token.Line {
					continue
				}
				// Next function begins after the last token of this function's body.
				endLine := 1 << 30
				for _, next := range program.Statements[position+1:] {
					if nextFn, ok := next.(*ast.FunctionDeclaration); ok {
						endLine = nextFn.Token.Line
						break
					}
				}
				if finding.Subject.Source.Line >= endLine {
					continue
				}
				switch finding.Rule {
				case PitfallWrongGuardSubject:
					got[0]++
				case PitfallCheckWithoutTransfer:
					got[1]++
				}
			}
			if got != expected {
				t.Errorf("depth %v %s: got %v, want %v", depth, function.Name.Value, got, expected)
			}
		}
		if err := a.SetPitfallBudget(0, 0); err != nil {
			t.Fatal(err)
		}
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, finding := range a.PitfallAnalysis().Results() {
			if finding.Rule == PitfallWrongGuardSubject || finding.Rule == PitfallCheckWithoutTransfer {
				t.Fatal("disabled advisory survived", finding)
			}
		}
	}
}
