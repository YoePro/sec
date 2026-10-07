package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestArenaInvalidationLSPDiagnostics preserves method spans, navigable live
// dependency locations, mandatory severity, identity and explicit Invalid proof.
// Rules: rules/memory/arena.md — §§119–120;
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestArenaInvalidationLSPDiagnostics(t *testing.T) {
	path := "../../testdata/sema/arena_invalidation_diagnostics_invalid.sec"
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
	if len(errs) != 6 {
		t.Fatal(errs)
	}
	for _, value := range errs {
		got := semaDiagnostic(value, 3, string(data))
		if got.Code != value.ID || got.Severity != 1 || !strings.HasPrefix(got.Message, "Invalid: ") || !strings.Contains(got.Message, value.Help) || len(got.RelatedInformation) != 1 || got.RelatedInformation[0].Message != "live Arena dependency" || got.RelatedInformation[0].Location.Range.Start.Line != value.PreviousLine-1 || got.Range.Start.Line != value.Line-1 || got.Range.Start.Character != value.Column-1 {
			t.Fatal(got)
		}
		width := 5
		if value.ID == diagnostics.ArenaReleaseLiveDependency {
			width = 7
		}
		if got.Range.End.Character-got.Range.Start.Character != width {
			t.Fatal(got)
		}
	}
}
