package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestPitfallDiagnosticOwnershipLSP keeps one mandatory occurrence, its exact
// range and related navigation while transporting the supporting explanation.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing";
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestPitfallDiagnosticOwnershipLSP(t *testing.T) {
	file := "../../testdata/sema/pitfall_diagnostic_ownership_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.NewWithFile(string(data), file)).ParseProgram()
	errors := sema.NewAnalyzer().Analyze(program)
	if len(errors) != 3 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		d := semaDiagnostic(e, 3, string(data))
		if d.Severity != 1 || d.Code != e.ID || strings.Count(d.Message, "pitfall.") != 1 || !strings.Contains(d.Message, "Suggested edit:") || d.Range.Start.Line != e.Line-1 {
			t.Fatal(d)
		}
		if e.PreviousFile != "" && len(d.RelatedInformation) != 1 {
			t.Fatal("missing canonical evidence navigation", d)
		}
	}
}

// TestPitfallActionsRetainPrimaryDiagnostic verifies that supporting actions
// preserve the exact primary range, proof, help and links under every optional
// policy, including an outer bool expression with an inner equality owner.
// Rules: rules/analysis/pitfall_analysis.md — Diagnostic ownership and coalescing;
// rules/tooling/lsp.md — Shared diagnostic model, Safe fixes.
func TestPitfallActionsRetainPrimaryDiagnostic(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/pitfall_diagnostic_ownership_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	primary := analyze(uri, text)
	if len(primary) != 3 {
		t.Fatal(primary)
	}
	for _, level := range []string{"off", "info", "warning", "error"} {
		actions := pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, pitfallDiagnosticSettings{Rules: map[string]string{diagnostics.PitfallAdvisory: level}})
		if len(actions) != 3 {
			t.Fatal(level, actions)
		}
		for _, action := range actions {
			if len(action.Diagnostics) != 1 || action.Disabled == nil || len(action.Edit.Changes) != 0 {
				t.Fatal(action)
			}
			matched := false
			for _, owner := range primary {
				matched = matched || reflect.DeepEqual(action.Diagnostics[0], owner)
			}
			if !matched {
				t.Fatalf("%s: fabricated action diagnostic %+v; primary %+v", level, action.Diagnostics[0], primary)
			}
		}
	}
}

// TestPitfallActionRejectsMissingOrStaleOwner requires exact snapshot identity.
// Rules: rules/analysis/pitfall_analysis.md — Diagnostic ownership and coalescing.
func TestPitfallActionRejectsMissingOrStaleOwner(t *testing.T) {
	source := lexer.Token{File: "cause.sec", Line: 1, Column: 4, EndLine: 1, EndColumn: 6}
	owner := sema.Error{ID: diagnostics.OperatorNonComparable, File: source.File, Line: source.Line, Column: source.Column, EndLine: source.EndLine, EndColumn: source.EndColumn}
	finding := sema.PitfallFinding{DiagnosticID: owner.ID, DiagnosticSource: source}
	if _, found := pitfallOwnerDiagnostic(finding, []sema.Error{owner}, "", "", sourceOverlay{}); !found {
		t.Fatal("exact owner missing")
	}
	for _, changed := range []lexer.Token{{}, {File: source.File, Line: 2, Column: source.Column}, {File: "another.sec", Line: source.Line, Column: source.Column}} {
		finding.DiagnosticSource = changed
		if _, found := pitfallOwnerDiagnostic(finding, []sema.Error{owner}, "", "", sourceOverlay{}); found {
			t.Fatal("stale or unrelated owner accepted", changed)
		}
	}
}

// TestSuppressedPitfallHasNoEditorDiagnosticOrAction keeps a compiler-proven
// empty guard as suppression evidence instead of a competing advisory.
// Rules: rules/analysis/pitfall_analysis.md — Suppressing evidence, Diagnostic ownership and coalescing.
func TestSuppressedPitfallHasNoEditorDiagnosticOrAction(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/pitfall_suppressed.sec")
	if err != nil {
		t.Fatal(err)
	}
	text, uri := string(data), uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	a := analyzeNavigationSource(uri, text)
	suppressed := 0
	for _, finding := range a.PitfallAnalysis().Results() {
		if finding.State == sema.PitfallStateSuppressed {
			suppressed++
		}
	}
	if suppressed == 0 {
		t.Fatal("fixture has no suppression evidence")
	}
	for _, level := range []string{"off", "info", "warning", "error"} {
		settings := pitfallDiagnosticSettings{Rules: map[string]string{diagnostics.PitfallAdvisory: level}}
		if values := pitfallDiagnostics(a, uri, text, sourceOverlay{}, settings); len(values) != 0 {
			t.Fatal(values)
		}
		if actions := pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, settings); len(actions) != 0 {
			t.Fatal(actions)
		}
	}
}
