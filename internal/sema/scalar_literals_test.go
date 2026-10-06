package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestUnicodeScalarLiteralDiagnostics verifies scalar-domain rejection and
// registered, explanatory, source-local diagnostics in ordinary and core code.
// Rules: rules/foundations/lexical_structure.md — §12.7 "Numeric family suffixes";
// rules/types/types.md — "rune"; rules/tooling/diagnostics.md — §5 and §7.
func TestUnicodeScalarLiteralDiagnostics(t *testing.T) {
	definition, registered := diagnostics.Lookup(diagnostics.InvalidUnicodeScalarLiteral)
	if !registered || definition.ID != "S1120" || definition.Name != "literal.invalid-unicode-scalar" || !definition.Mandatory || definition.DefaultSeverity != diagnostics.SeverityError || definition.Retired {
		t.Fatal("missing mandatory scalar diagnostic definition", definition)
	}
	for _, trusted := range []bool{false, true} {
		for _, fixture := range []struct {
			file   string
			errors int
		}{
			{"unicode_scalar_literals_valid.sec", 0},
			{"unicode_scalar_literals_invalid.sec", 5},
		} {
			source, err := os.ReadFile("../../testdata/sema/" + fixture.file)
			if err != nil {
				t.Fatal(err)
			}
			file := "testdata/" + fixture.file
			p := parser.New(lexer.NewWithFile(string(source), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if trusted {
				program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
			}
			errors := NewAnalyzer().Analyze(program)
			if len(errors) != fixture.errors {
				t.Fatalf("%s core=%v: got %v", fixture.file, trusted, errors)
			}
			for _, diagnostic := range errors {
				if diagnostic.ID != diagnostics.InvalidUnicodeScalarLiteral || diagnostic.Severity != diagnostics.SeverityError {
					t.Fatal("invalid scalar emitted unregistered/wrong severity diagnostic", diagnostic)
				}
				if !strings.Contains(diagnostic.Message, "U+0000..U+10FFFF") || !strings.Contains(diagnostic.Message, "surrogates U+D800..U+DFFF") || !strings.Contains(diagnostic.Help, "0xD800u..0xDFFFu") {
					t.Fatal("missing domain explanation or integer-codepoint help", diagnostic)
				}
				if diagnostic.File != file || diagnostic.Line < 3 || diagnostic.Column <= 0 || diagnostic.EndColumn <= diagnostic.Column {
					t.Fatal("scalar diagnostic lost literal source span", diagnostic)
				}
			}
		}
	}
}
