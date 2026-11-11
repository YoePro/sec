package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// LSP publishes canonical raw-pointer mentor diagnostics and causal locations,
// without a pointer proof model in the editor or an automatic unsafe rewrite.
// Rules: rules/memory/raw_pointers.md §§34–35; rules/tooling/lsp.md — Shared diagnostic model.
func TestRawPointerMentorLSPDiagnostics(t *testing.T) {
	for _, name := range []string{"raw_pointer_mentor_invalid", "raw_pointer_mentor_valid"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/tooling/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			counts := map[string]int{}
			for _, d := range analyze(uri, string(data)) {
				if d.Severity != 1 {
					continue
				}
				if name == "raw_pointer_mentor_valid" {
					t.Fatal(d)
				}
				counts[d.Code]++
				if !strings.Contains(d.Message, "help:") {
					t.Fatalf("lost mentor help: %+v", d)
				}
				if strings.HasPrefix(d.Message, "RawPtr.Read requires") || strings.HasPrefix(d.Message, "RawPtr.Write requires") {
					if strings.Contains(d.Message, "volatile") {
						t.Fatalf("ordinary access acquired volatile caller obligations: %+v", d)
					}
				}
				if d.Code == diagnostics.RawPointerNullDereference || d.Code == diagnostics.RawPointerDanglingOrigin {
					if len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.URI != uri {
						t.Fatalf("lost causal source: %+v", d)
					}
				}
			}
			if name == "raw_pointer_mentor_invalid" && (len(counts) != 4 || counts[diagnostics.RawPointerUnsafeRequired] != 11 || counts[diagnostics.RawPointerInvalidOperation] != 21 || counts[diagnostics.RawPointerNullDereference] != 6 || counts[diagnostics.RawPointerDanglingOrigin] != 1) {
				t.Fatal(counts)
			}
		})
	}
}

// Sema owns validity, mentor help and causal origins; unsaved document and
// newline changes must preserve the complete LSP projection of those facts.
// Rules: rules/memory/raw_pointers.md §§34–35;
// rules/tooling/lsp.md §10.1(1–3) shared diagnostic model.
func TestRawPointerMentorCanonicalProjection(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_mentor_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, text string }{
		{"LF", string(data)},
		{"CRLF", strings.ReplaceAll(string(data), "\n", "\r\n")},
		{"UTF16", strings.Replace(string(data), "let read := pointer.Read()", "/* 😀 */ let read := pointer.Read()", 1)},
		{"unsaved-operation-edit", strings.Replace(string(data), "pointer.Read()", "pointer.VolatileRead()", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "main.sec")
			uri := uriFromPath(path)
			p := parser.New(lexer.NewWithFile(tc.text, path))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			errors := sema.NewAnalyzer().Analyze(program)
			if len(errors) != 39 {
				t.Fatalf("changed safety matrix: %v", errors)
			}
			var got []diagnostic
			for _, d := range analyze(uri, tc.text) {
				if d.Severity == 1 {
					got = append(got, d)
				}
			}
			if len(got) != len(errors) {
				t.Fatalf("LSP has %d errors, Sema has %d", len(got), len(errors))
			}
			// Convert the canonical scalar columns independently of the LSP
			// diagnostic renderer, including the non-BMP comment above.
			lines := strings.Split(tc.text, "\n")
			protocolPosition := func(line, column int) position {
				prefix := []rune(lines[line-1])[:column-1]
				return position{Line: line - 1, Character: len(utf16.Encode(prefix))}
			}
			for i, e := range errors {
				d := got[i]
				wantRange := lspRange{Start: protocolPosition(e.Line, e.Column), End: protocolPosition(e.EndLine, e.EndColumn)}
				message := e.DisplayMessage()
				links := e.DiagnosticProvenance().Related
				for _, link := range links {
					message += fmt.Sprintf("\n\n%s at %s:%d:%d", link.Label, link.File, link.Line, link.Column)
				}
				if e.Help != "" {
					message += "\n\nhelp: " + e.Help
				}
				if d.Code != e.ID || d.Range != wantRange || d.Message != message || d.Source != "sec" || len(d.RelatedInformation) != len(links) {
					t.Fatalf("occurrence %d lost canonical fields: %+v; Sema %+v", i, d, e)
				}
				for j, link := range links {
					want := lspRange{Start: protocolPosition(link.Line, link.Column), End: protocolPosition(link.EndLine, link.EndColumn)}
					related := d.RelatedInformation[j]
					if related.Message != link.Label || related.Location.URI != uri || related.Location.Range != want {
						t.Fatalf("lost causal origin: %+v; Sema %+v", related, link)
					}
				}
			}
		})
	}
}
