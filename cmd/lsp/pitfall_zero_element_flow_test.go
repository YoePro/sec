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

// Only unsuppressed skipped-zero intent advice reaches the editor;
// suggested changes remain disabled and never become automatic edits.
// Rules: rules/analysis/pitfall_analysis.md — "LSP presentation", "Fix safety".
func TestLSPZeroElementFlow(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_zero_element_flow.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte("[analysis]\nlsp_depth = 'standard'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "zero.sec")
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
		if strings.Contains(value.Message, string(sema.PitfallSkippedFirstElement)) {
			count++
		}
	}
	if count != 16 {
		t.Fatalf("skipped-zero diagnostics = %d, want 16", count)
	}
	count = 0
	for _, action := range pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if !strings.Contains(action.Title, "start complete traversal at index 0") {
			continue
		}
		count++
		if action.Disabled == nil || len(action.Edit.Changes) != 0 {
			t.Fatal("intent edit became automatic", action)
		}
	}
	if count != 16 {
		t.Fatalf("skipped-zero actions = %d, want 16", count)
	}
}
