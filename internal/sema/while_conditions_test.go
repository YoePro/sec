package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Mandatory bool-condition validity is invariant under optional analysis depth;
// successful try payloads remain bool and failures never supply truthiness.
// Rules: rules/control-flow/flowcontrol_while.md — §§3–7,28,30;
// rules/compiler/compiler_analysis.md — mandatory checks at every depth.
func TestWhileConditionConformanceAtEveryDepth(t *testing.T) {
	for _, name := range []string{"effects", "non_bool_invalid"} {
		input, err := os.ReadFile("../../testdata/control_flow/while_conditions/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(input), name+".sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			errors := a.Analyze(program)
			want := 0
			if name != "effects" {
				want = 2
			}
			if len(errors) != want {
				t.Fatalf("%s/%v: %v", name, depth, errors)
			}
			for _, err := range errors {
				if !strings.Contains(err.Message, "while condition must") || !strings.Contains(err.Message, "bool, got int") {
					t.Fatal(err)
				}
			}
		}
	}
}
