package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestExplicitDiscardAdvice keeps optional style guidance separate from actual
// ownership effects, unknown calls and nontrivial destruction. Loop visits and
// repeated analysis produce stable occurrences at every depth/policy level.
// Rules: rules/control-flow/discard.md — §§7,9,13,27,37–38;
// rules/memory/destruction.md — §3.3(2); diagnostics.md — §18.
func TestExplicitDiscardAdvice(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/explicit_discard_advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, level := range []string{"default", "off", "info", "warning", "error"} {
			p := parser.New(lexer.NewWithFile(string(source), "explicit.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if level != "default" {
				a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.useless-explicit-discard": level, "ownership.redundant-void-discard": level})
			}
			for attempt := 0; attempt < 2; attempt++ {
				errors := a.Analyze(program)
				facts := a.Warnings()
				severity := diagnostics.SeverityInformation
				if level == "error" {
					facts = errors
					severity = diagnostics.SeverityError
				} else if len(errors) != 0 {
					t.Fatal(errors)
				}
				if level == "warning" {
					severity = diagnostics.SeverityWarning
				}
				want := strings.Count(string(source), "// useless") + strings.Count(string(source), "// void")
				if level == "off" {
					want = 0
				}
				if len(facts) != want {
					t.Fatalf("%s/%s: want %d: %v", depth, level, want, facts)
				}
				lines := strings.Split(string(source), "\n")
				for _, fact := range facts {
					expected := diagnostics.UselessExplicitDiscard
					if strings.Contains(lines[fact.Line-1], "// void") {
						expected = diagnostics.RedundantVoidDiscard
					}
					if (!strings.Contains(lines[fact.Line-1], "// useless") && !strings.Contains(lines[fact.Line-1], "// void")) || fact.ID != expected || fact.Severity != severity || fact.Help == "" || fact.EndColumn-fact.Column != 7 {
						t.Fatalf("incorrect advice: %+v", fact)
					}
				}
			}
		}
	}
}
