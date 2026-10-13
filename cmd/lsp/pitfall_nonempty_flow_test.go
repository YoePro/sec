package main

import (
	"os"
	"path/filepath"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestLSPNonEmptyProofs transports only unsuppressed compiler-owned findings.
// Rules: rules/tooling/lsp.md — Shared diagnostic model;
// rules/analysis/pitfall_analysis.md — Final-element access requires non-empty proof.
func TestLSPNonEmptyProofs(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/pitfall_nonempty_flow.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "nonempty.sec")
	text := string(data)
	p := parser.New(lexer.NewWithFile(text, path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	values := pitfallDiagnostics(a, uriFromPath(path), text, sourceOverlay{}, pitfallDiagnosticSettings{})
	count := 0
	for _, value := range values {
		if strings.Contains(value.Message, string(sema.PitfallFinalElementNeedsNonEmpty)) {
			count++
		}
	}
	if count != 7 {
		t.Fatal(values)
	}
}
