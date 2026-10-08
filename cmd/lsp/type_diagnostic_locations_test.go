package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestTypeDiagnosticLocationsLSP transports Sema's cross-file type provenance
// using each file's own overlay and UTF-16 coordinates, preserving IDs/severity.
// Rules: rules/types/contracts.md — Diagnostics; rules/types/default_values.md — Diagnostics;
// rules/tooling/lsp.md — Shared diagnostic model, protocol position encoding.
func TestTypeDiagnosticLocationsLSP(t *testing.T) {
	program := &ast.Program{}
	overlay := sourceOverlay{Sources: map[string]string{}}
	for _, name := range []string{"definitions.sec", "use_invalid.sec"} {
		path, err := filepath.Abs("../../testdata/sema/type_locations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// An unsaved comment puts non-BMP text before declarations without changing
		// the contract/default source forms or line ownership.
		text := strings.Replace(string(data), "type Base int range", "/* 😀 */ type Base int range", 1)
		p := parser.New(lexer.NewWithFile(text, path))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, parsed.Statements...)
		overlay.Sources[path] = text
	}
	a := sema.NewAnalyzer()
	errors := a.Analyze(program)
	if len(errors) != 13 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		uri := uriFromPath(e.File)
		text := overlay.Sources[e.File]
		d := semaDiagnosticWithSources(e, 3, uri, text, overlay)
		if d.Code != e.ID || d.Severity != 1 || len(d.RelatedInformation) != 1 {
			t.Fatal(e, d)
		}
		link := d.RelatedInformation[0]
		relatedURI := uriFromPath(e.PreviousFile)
		related := overlay.Sources[e.PreviousFile]
		token := lexer.Token{Line: e.PreviousLine, Column: e.PreviousColumn}
		if e.RelatedLabel == "contract declaration" && e.PreviousLine == 3 {
			line := strings.Split(related, "\n")[2]
			index := strings.Index(line, "even")
			want := len(utf16.Encode([]rune(line[:index])))
			if link.Location.Range.Start.Character != want {
				t.Fatalf("UTF-16 related column=%d, want %d", link.Location.Range.Start.Character, want)
			}
		}
		if link.Location.URI != relatedURI || link.Location.Range.Start != diagnosticTokenStart(related, token) || link.Message != e.RelatedLabel {
			t.Fatal("source relation changed", e, d)
		}
	}
}
