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
			{"unicode_scalar_literals_invalid.sec", 4},
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

// TestCharRuneLiteralSemantics verifies the MD-043 literal model: single-quoted
// literals are rune by default regardless of value, an explicit char context
// shapes only scalars 0..255, t literals are limited to 0..255, unsuffixed
// integers never shape to char, and byte/char/rune stay distinct.
// Rules: rules/types/types.md — "byte", "char", "rune", "Character literal";
// rules/foundations/lexical_structure.md — §12.7, §13;
// rules/corrections/applied/md043-char-rune-literal-correction-20261008.md — §6.
func TestCharRuneLiteralSemantics(t *testing.T) {
	definition, registered := diagnostics.Lookup(diagnostics.CharLiteralOutOfRange)
	if !registered || definition.ID != "S1138" || definition.Name != "literal.char-out-of-range" || !definition.Mandatory {
		t.Fatal("missing mandatory char-domain diagnostic definition", definition)
	}

	valid, err := os.ReadFile("../../testdata/sema/char_rune_literals_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(valid), "char_rune_literals_valid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(program), nil)
	inferred := map[string]TypeKind{}
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Name == nil || function.Name.Value != "InferredRunes" {
			continue
		}
		for _, inner := range function.Body.Statements {
			if let, ok := inner.(*ast.LetStatement); ok && let.Type == nil {
				inferred[let.Name.Value] = analyzer.expressionTypes[let.Value].Kind
			}
		}
	}
	if inferred["letter"] != RuneType || inferred["pi"] != RuneType {
		t.Fatalf("inferred character literal kinds = %v, want rune for both ASCII and non-Latin-1 literals", inferred)
	}

	invalid, err := os.ReadFile("../../testdata/sema/char_rune_literals_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(invalid))
	assertSemaErrors(t, errors, []string{
		"value 256t does not fit char; t-suffixed literals must be in 0..255 at 3:35",
		"value 300t does not fit char; t-suffixed literals must be in 0..255 at 4:38",
		"character literal 'Ω' does not fit char; char holds values 0..255 at 5:32",
		"cannot initialize char with int at 7:28",
		"cannot initialize char with byte at 11:28",
		"cannot initialize byte with char at 15:22",
		"cannot initialize char with rune at 20:28",
	})
	for _, index := range []int{0, 1, 2} {
		if errors[index].ID != diagnostics.CharLiteralOutOfRange || errors[index].Help == "" {
			t.Fatalf("char-domain error %d = %#v, want S1138 with repair help", index, errors[index])
		}
	}
	if !strings.Contains(errors[3].Help, "65t") || !strings.Contains(errors[3].Help, "char(65)") {
		t.Fatalf("unsuffixed integer help = %q, want 65t and char(65)", errors[3].Help)
	}
}
