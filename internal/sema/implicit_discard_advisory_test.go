package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestImplicitDiscardedResultPolicy checks all depths/severities, repeated
// analysis and loop/defer probes. Explicit acknowledgement and value consumers
// remain distinct from standalone ordinary function/method/generic calls.
// Rules: rules/control-flow/discard.md — §§10–13,37–38;
// rules/tooling/diagnostics.md — §18.
func TestImplicitDiscardedResultPolicy(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/implicit_discard.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, level := range []string{"default", "off", "info", "warning", "error"} {
			p := parser.New(lexer.NewWithFile(string(source), "implicit.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if level != "default" {
				a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.implicit-discarded-result": level})
			}
			var previous []Error
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
				want := strings.Count(string(source), "// advisory")
				if level == "off" {
					want = 0
				}
				if len(facts) != want {
					t.Fatalf("%s/%s want %d: %v", depth, level, want, facts)
				}
				lines := strings.Split(string(source), "\n")
				for _, fact := range facts {
					if fact.ID != diagnostics.ImplicitDiscardedResult || fact.Severity != severity || fact.Help == "" || fact.EndColumn <= fact.Column || !strings.Contains(lines[fact.Line-1], "// advisory") {
						t.Fatalf("incorrect advisory: %+v", fact)
					}
					if !strings.Contains(lines[fact.Line-1], "int(4)") && !strings.Contains(lines[fact.Line-1], "new Counter()") && (fact.PreviousLine == 0 || fact.RelatedLabel != "selected callable declaration") {
						t.Fatalf("missing callable provenance: %+v", fact)
					}
				}
				if attempt == 0 {
					previous = facts
				} else if !reflect.DeepEqual(previous, facts) {
					t.Fatal("unstable repeated analysis")
				}
			}
		}
	}
}

// TestImplicitDiscardPolicyKeepsMustUseErrors checks that mandatory Result
// obligations cannot be suppressed and invalid calls do not produce advice.
// Rules: rules/control-flow/discard.md — §§12–13,38.
func TestImplicitDiscardPolicyKeepsMustUseErrors(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/implicit_discard_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"off", "info", "warning", "error"} {
		p := parser.New(lexer.New(string(source)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.implicit-discarded-result": level, "ownership.unhandled-must-use-result": "off"})
		errors := a.Analyze(program)
		count := 0
		constructionRejected := false
		for _, err := range errors {
			constructionRejected = constructionRejected || strings.Contains(err.Message, "fallible init with construction error")
			if err.ID == diagnostics.UnhandledMustUseResult {
				count++
			}
			if err.ID == diagnostics.ImplicitDiscardedResult {
				t.Fatalf("invalid call got advisory: %+v", err)
			}
		}
		if count != 2 || !constructionRejected {
			t.Fatalf("%s suppressed must-use errors: %v", level, errors)
		}
		for _, warning := range a.Warnings() {
			if warning.ID == diagnostics.ImplicitDiscardedResult {
				t.Fatal(warning)
			}
		}
	}
}
