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

// TestImplicitEmptyCatchAllAdvisory keeps optional warning policy independent
// of match legality and flow. Explicit empty arms (including guarded ones),
// initialized subjects and refined branches do not receive the advisory.
// Rules: rules/declarations/unions.md — §§10.2–10.4,11;
// rules/tooling/diagnostics.md — §18.
func TestImplicitEmptyCatchAllAdvisory(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/union_empty_catch_all.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, level := range []string{"default", "off", "info", "warning", "error"} {
			p := parser.New(lexer.NewWithFile(string(data), "empty.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if level != "default" {
				a.SetDiagnosticRules(diagnostics.RulePolicy{"union.implicit-empty-catch-all": level})
			}
			var previous []Error
			for attempt := 0; attempt < 2; attempt++ {
				errors := a.Analyze(program)
				if level != "error" && len(errors) != 0 {
					t.Fatal(depth, level, errors)
				}
				occurrences := a.Warnings()
				if level == "error" {
					occurrences = errors
				}
				facts := []Error{}
				for _, fact := range occurrences {
					if fact.ID == diagnostics.ImplicitEmptyCatchAll {
						facts = append(facts, fact)
					}
				}
				want := 3
				if level == "off" {
					want = 0
				}
				if len(facts) != want {
					t.Fatalf("%s %s: want %d, got %v", depth, level, want, occurrences)
				}
				severity := diagnostics.SeverityWarning
				if level == "info" {
					severity = diagnostics.SeverityInformation
				}
				if level == "error" {
					severity = diagnostics.SeverityError
				}
				for _, fact := range facts {
					if fact.Severity != severity || fact.Help == "" || fact.Line < 1 || fact.EndColumn-fact.Column != 1 || !strings.Contains(fact.Message, "possible empty state of state") {
						t.Fatal(fact)
					}
				}
				if attempt == 0 {
					previous = facts
				} else if !reflect.DeepEqual(previous, facts) {
					t.Fatal("unstable repeated analysis", previous, facts)
				}
			}
		}
	}
}

// TestImplicitEmptyCatchAllPolicyPreservesExhaustiveness confirms that turning
// off optional guidance does not waive the required reachable empty-state arm.
// A guarded wildcard alone is not unconditional coverage and gets no A2009.
// Rules: rules/declarations/unions.md — §§10.2,10.4,11;
// rules/tooling/diagnostics.md — §18.
func TestImplicitEmptyCatchAllPolicyPreservesExhaustiveness(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/union_empty_catch_all_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	a.SetDiagnosticRules(diagnostics.RulePolicy{"union.implicit-empty-catch-all": "off"})
	errors := a.Analyze(program)
	found := false
	for _, err := range errors {
		found = found || strings.Contains(err.Message, "possibly empty state")
	}
	if !found {
		t.Fatalf("disabled guidance waived mandatory empty coverage: %v", errors)
	}
	for _, fact := range a.Warnings() {
		if fact.ID == diagnostics.ImplicitEmptyCatchAll {
			t.Fatal("guarded wildcard received advisory", fact)
		}
	}
}
