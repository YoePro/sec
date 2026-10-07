package main

import (
	"os"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestEscapeDiagnosticsLSP preserves mandatory escape identities, source sinks,
// origin navigation and remedies even with a configured warning severity.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic quality";
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestEscapeDiagnosticsLSP(t *testing.T) {
	path := "../../testdata/sema/escape_diagnostics_invalid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errs := sema.NewAnalyzer().Analyze(program)
	if len(errs) != 9 {
		t.Fatal(errs)
	}
	for _, e := range errs {
		got := semaDiagnostic(e, 3, string(data))
		if !strings.Contains(got.Message, "note:") || got.Code != e.ID || got.Severity != 1 || !strings.Contains(got.Message, e.Help) || got.Range.Start.Line != e.Line-1 || got.Range.Start.Character != e.Column-1 || got.Range.End.Line != e.EndLine-1 || got.Range.End.Character != e.EndColumn-1 {
			t.Fatal(got)
		}
		if e.ID != diagnostics.EscapeVariadicPack && (len(got.RelatedInformation) < 1 || got.RelatedInformation[0].Location.Range.Start.Line != e.PreviousLine-1 || got.RelatedInformation[0].Location.Range.Start.Character != e.PreviousColumn-1) {
			t.Fatal(got)
		}
	}
}
