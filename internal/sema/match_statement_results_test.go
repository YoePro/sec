package sema

import (
	"os"
	"reflect"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestStatementMatchResultsRequireHandling checks that a statement arm cannot
// hide a must-use Result returned by an ordinary call or carrier constructor.
// Rules: rules/control-flow/flowcontrol_match.md §19(2)-(6);
// rules/control-flow/discard.md §§10-11,14.
func TestStatementMatchResultsRequireHandling(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/match_statement_results_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := NewAnalyzer()
	analyzer.SetDiagnosticRules(diagnostics.RulePolicy{
		"ownership.unhandled-must-use-result": "off",
		"ownership.non-discardable-value":     "off",
		"ownership.implicit-discarded-result": "off",
	})
	errors := analyzer.Analyze(program)
	if len(errors) != 12 {
		t.Fatalf("want 12 mandatory result errors, got %+v", errors)
	}
	for _, err := range errors {
		if (err.ID != diagnostics.UnhandledMustUseResult && err.ID != diagnostics.NonDiscardableValue) || err.Help == "" {
			t.Errorf("missing registered must-use error: %+v", err)
		}
	}
}

// TestStatementMatchResultsShareDiscardPolicy verifies that loop/backedge
// visits retain one advisory per source expression and that advisory policy
// does not change value/return arms or explicit acknowledgement.
// Rules: rules/control-flow/flowcontrol_match.md §19(2)-(6);
// rules/control-flow/discard.md §§10-13,38.
func TestStatementMatchResultsShareDiscardPolicy(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/match_statement_results_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, level := range []string{"default", "off", "warning", "error"} {
			t.Run(string(depth)+"/"+level, func(t *testing.T) {
				p := parser.New(lexer.New(source))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithDepth(depth)
				if level != "default" {
					a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.implicit-discarded-result": level})
				}
				var previous []Error
				for repeat := 0; repeat < 2; repeat++ {
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
					want := 3
					if level == "off" {
						want = 0
					}
					if len(facts) != want {
						t.Fatalf("want %d advisories, got %+v", want, facts)
					}
					lines := strings.Split(source, "\n")
					for _, fact := range facts {
						if fact.ID != diagnostics.ImplicitDiscardedResult || fact.Severity != severity || !strings.Contains(lines[fact.Line-1], "// advisory") || fact.PreviousLine == 0 {
							t.Errorf("incorrect arm-result advisory: %+v", fact)
						}
					}
					if repeat > 0 && !reflect.DeepEqual(previous, facts) {
						t.Fatal("unstable repeated analysis")
					}
					previous = facts
				}
			})
		}
	}
}
