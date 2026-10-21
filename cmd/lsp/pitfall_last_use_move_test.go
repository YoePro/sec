package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Final-use move hints reach the optional LSP diagnostic and remain manual
// suggestions with no workspace edit; no parallel ownership analysis lives here.
// Rules: rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use", "LSP presentation", "Fix safety".
func TestPitfallLastUseMoveLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_move/arguments.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	p := parser.New(lexer.NewWithFile(text, pathFromURI(uri)))
	a := sema.NewAnalyzer()
	if errs := a.Analyze(p.ParseProgram()); len(errs) != 0 {
		t.Fatal(errs)
	}
	found := 0
	for _, d := range pitfallDiagnostics(a, uri, text, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if !strings.Contains(d.Message, string(sema.PitfallLastUseArgumentCopy)) {
			continue
		}
		found++
		if d.Code != diagnostics.PitfallAdvisory || d.Severity != 3 || !strings.Contains(d.Message, "<-files") {
			t.Fatal(d)
		}
	}
	if found != 7 {
		t.Fatalf("move diagnostics=%d want 7", found)
	}
	suggested := 0
	for _, action := range pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if !strings.Contains(action.Title, "moving files") {
			continue
		}
		suggested++
		if action.Disabled == nil || len(action.Edit.Changes) != 0 {
			t.Fatal("uncertified ownership edit became applicable", action)
		}
	}
	if suggested != 7 {
		t.Fatalf("manual actions=%d want 7", suggested)
	}
	for _, d := range pitfallDiagnostics(a, uri, text, sourceOverlay{}, pitfallDiagnosticSettings{Rules: map[string]string{"A2004": "off"}}) {
		if strings.Contains(d.Message, string(sema.PitfallLastUseArgumentCopy)) {
			t.Fatal("optional move advice ignored off policy")
		}
	}
}
