package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Inclusive endpoint suggestions remain disabled intent actions; only
// unsuppressed cross-collection advice reaches editor diagnostics.
// Rules: rules/analysis/pitfall_analysis.md — "LSP presentation", "Fix safety".
func TestLSPInclusiveBoundSource(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_inclusive_bound_source.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	path := filepath.Join(t.TempDir(), "capacity.sec")
	uri := uriFromPath(path)
	p := parser.New(lexer.NewWithFile(text, path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	count := 0
	for _, value := range pitfallDiagnostics(a, uri, text, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if strings.Contains(value.Message, string(sema.PitfallWrongBoundSource)) {
			count++
		}
	}
	if count != 13 {
		t.Fatalf("bound-source diagnostics = %d, want 13", count)
	}
	count = 0
	for _, action := range pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if !strings.Contains(action.Title, "use a half-open range bounded by") {
			continue
		}
		count++
		if action.Disabled == nil || len(action.Edit.Changes) != 0 {
			t.Fatal("intent edit became automatic", action)
		}
	}
	if count != 13 {
		t.Fatalf("bound-source actions = %d, want 13", count)
	}
}
