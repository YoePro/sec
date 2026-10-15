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

// Only unsuppressed compiler-owned capacity advisories reach editor diagnostics
// and code actions; intent suggestions never become automatic text edits.
// Rules: rules/analysis/pitfall_analysis.md — "LSP presentation", "Fix safety";
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestLSPCapacityProofFlow(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_flow.sec")
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
		if strings.Contains(value.Message, string(sema.PitfallCapacityAsLength)) {
			count++
		}
	}
	if count != 19 {
		t.Fatalf("capacity diagnostics = %d, want 19", count)
	}
	count = 0
	for _, action := range pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if !strings.Contains(action.Title, "index relative to the live length Len") {
			continue
		}
		count++
		if action.Disabled == nil || len(action.Edit.Changes) != 0 {
			t.Fatal("intent edit became automatic", action)
		}
	}
	if count != 19 {
		t.Fatalf("capacity actions = %d, want 19", count)
	}
}

// Only the five unsupported live-length assumptions become editor advisories;
// ten proven-safe sites and eight unknown/near-miss sites offer no action.
// Rules: rules/analysis/pitfall_analysis.md — "LSP presentation", "Fix safety".
func TestLSPCapacityWhile(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_while.sec")
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
		if strings.Contains(value.Message, string(sema.PitfallCapacityAsLength)) {
			count++
		}
	}
	if count != 5 {
		t.Fatalf("capacity diagnostics = %d, want 5", count)
	}
	count = 0
	for _, action := range pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if !strings.Contains(action.Title, "bound the while traversal by live Len") {
			continue
		}
		count++
		if action.Disabled == nil || len(action.Edit.Changes) != 0 {
			t.Fatal("intent edit became automatic", action)
		}
	}
	if count != 5 {
		t.Fatalf("capacity actions = %d, want 5", count)
	}
}

// Expression-local proof evidence removes editor advice at guarded RHS sites,
// while late checks and inclusive capacity endpoints retain suggested actions.
// Rules: rules/analysis/pitfall_analysis.md — "LSP presentation", "Fix safety".
func TestLSPCapacityExpressions(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_capacity_expressions.sec")
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
		if strings.Contains(value.Message, string(sema.PitfallCapacityAsLength)) {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("capacity diagnostics = %d, want 3", count)
	}
	count = 0
	for _, action := range pitfallCodeActions(uri, text, lspRange{End: endPosition(text)}, sourceOverlay{}, pitfallDiagnosticSettings{}) {
		if !strings.Contains(action.Title, "bound the while traversal by live Len") {
			continue
		}
		count++
		if action.Disabled == nil || len(action.Edit.Changes) != 0 {
			t.Fatal("intent edit became automatic", action)
		}
	}
	if count != 3 {
		t.Fatalf("capacity actions = %d, want 3", count)
	}
}
