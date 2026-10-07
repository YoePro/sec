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

// TestPitfallDiagnosticOwnership requires one normative owner per operation,
// including an inner equality presented as an outer conversion, immutable
// evidence, admission gating, namespace reset and cause-specific matching.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing",
// "Analysis states", "Source mapping", "Fix safety".
func TestPitfallDiagnosticOwnership(t *testing.T) {
	file := "../../testdata/sema/pitfall_diagnostic_ownership_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		errors := a.Analyze(program)
		if len(errors) != 3 {
			t.Fatal(errors)
		}
		for _, e := range errors {
			if e.ID != diagnostics.OperatorNonComparable && e.ID != diagnostics.IndexOutOfBounds {
				t.Fatal(e)
			}
			if strings.Count(e.Help, "pitfall.") != 1 || !strings.Contains(e.Help, "Suggested edit:") {
				t.Fatal(e)
			}
		}
		findings := a.PitfallAnalysis().Findings()
		if len(findings) != 3 {
			t.Fatal(findings)
		}
		for _, f := range findings {
			if f.DiagnosticID == "" || f.State != PitfallStateFinding {
				t.Fatal(f)
			}
			for _, action := range f.Actions {
				if action.Kind != PitfallSuggestedEdit {
					t.Fatal(action)
				}
			}
		}
		if !strings.HasPrefix(findings[1].Subject.Expression, "bool(") || findings[1].DiagnosticID != diagnostics.OperatorNonComparable {
			t.Fatal(findings[1])
		}
		before := append([]Error(nil), a.errors...)
		for _, f := range findings {
			a.coalescePitfallDiagnostic(&f, f.Subject.Source)
		}
		if !reflect.DeepEqual(before, a.errors) {
			t.Fatal("repeated enrichment changed the primary occurrences")
		}
		if err := a.SetPitfallBudget(0, 0); err != nil {
			t.Fatal(err)
		}
		errors = a.Analyze(program)
		if len(errors) != 3 {
			t.Fatal("advisory budget disabled normative errors", errors)
		}
		for _, e := range errors {
			if e.ID == diagnostics.OperatorNonComparable && strings.Contains(e.Help, "pitfall.") {
				t.Fatal("disabled advisory enriched its owner", e)
			}
		}
		empty := parser.New(lexer.New("module empty\nfn Empty() void {}\n")).ParseProgram()
		if errors := a.Analyze(empty); len(errors) != 0 || len(a.pitfallDiagnosticOwners) != 0 {
			t.Fatal("stale diagnostic owners survived reuse", errors)
		}
	}
	// Equal source positions do not make unrelated semantic causes identical.
	a := NewAnalyzer()
	token := lexer.Token{File: "causes.sec", Line: 4, Column: 5}
	a.errors = []Error{{ID: diagnostics.OperatorNonComparable, Help: "original"}}
	a.pitfallDiagnosticOwners = map[pitfallRootCause]int{{sourceTokenLocation(token), "equality-type-compatibility"}: 0}
	finding := PitfallFinding{Rule: PitfallDirectIndexAtLength, OwningRule: "bounds", Subject: PitfallSubject{Source: token}}
	a.coalescePitfallDiagnostic(&finding, token)
	if finding.DiagnosticID != "" || a.errors[0].Help != "original" {
		t.Fatal("unrelated error claimed the finding")
	}
	finding.OwningRule, finding.State = "equality-type-compatibility", PitfallStateSuppressed
	a.coalescePitfallDiagnostic(&finding, token)
	if finding.DiagnosticID != "" || a.errors[0].Help != "original" {
		t.Fatal("suppression changed a mandatory error")
	}
}
