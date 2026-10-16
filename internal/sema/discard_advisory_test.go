package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestRedundantDiscardAdvisory keeps discard semantics independent of advice:
// only definite no-ops are reported, conditional/reinitialized/dead/probe paths
// are excluded, and all policy levels preserve identity and ownership results.
// Rules: rules/control-flow/discard.md — §§5,26,37–38;
// rules/memory/destruction.md — §§12.2(3),33.2(4).
func TestRedundantDiscardAdvisory(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/redundant_discard_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, level := range []string{"default", "off", "info", "warning", "error"} {
			p := parser.New(lexer.NewWithFile(string(data), "redundant.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if level != "default" {
				a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.redundant-discard": level})
			}
			var original []Error
			for attempt := 0; attempt < 2; attempt++ {
				errs := a.Analyze(program)
				if level == "error" {
					if len(errs) != 3 {
						t.Fatal(depth, level, errs)
					}
				} else if len(errs) != 0 {
					t.Fatal(depth, level, errs)
				}
				facts := a.Warnings()
				if level == "error" {
					facts = errs
				}
				if level == "off" {
					if len(facts) != 0 {
						t.Fatal(facts)
					}
					continue
				}
				if len(facts) != 3 {
					t.Fatal(depth, level, facts)
				}
				want := diagnostics.SeverityInformation
				if level == "warning" {
					want = diagnostics.SeverityWarning
				}
				if level == "error" {
					want = diagnostics.SeverityError
				}
				for _, fact := range facts {
					if fact.ID != diagnostics.RedundantDiscard || fact.Severity != want || fact.PreviousLine == 0 || fact.RelatedLabel == "" || fact.Help == "" {
						t.Fatal(fact)
					}
				}
				if attempt == 0 {
					original = facts
				} else if !reflect.DeepEqual(original, facts) {
					t.Fatal("unstable advisory", original, facts)
				}
			}
		}
	}
}

// TestRedundantDiscardPolicyCannotDisableOwnershipError checks that advice
// never revives an unavailable value or disables a mandatory use-after error.
// Rules: rules/control-flow/discard.md — §§5,38.
func TestRedundantDiscardPolicyCannotDisableOwnershipError(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/redundant_discard_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"off", "info", "error"} {
		p := parser.New(lexer.New(string(data)))
		a := NewAnalyzer()
		a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.redundant-discard": level, diagnostics.UseAfterDiscard: "off"})
		errs := a.Analyze(p.ParseProgram())
		if len(errs) != 2 || errs[0].ID != diagnostics.UseAfterDiscard || len(a.Warnings()) != 0 {
			t.Fatal(errs, a.Warnings())
		}
	}
}
