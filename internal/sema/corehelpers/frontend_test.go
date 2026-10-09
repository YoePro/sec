package corehelpers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/formatter"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// load checks the ordinary Sec lexical/AST form and formatting fixed point;
// helper names keep their source owner without introducing a new body syntax.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.3, 21–22;
// rules/library/core-library.md — §1.2 private helpers.
func load(t *testing.T, names ...string) *ast.Program {
	t.Helper()
	program := &ast.Program{SourceProvenance: map[string]ast.SourceProvenance{}}
	for _, name := range names {
		path := filepath.Join("../../../testdata/core/private_helpers", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		formatted := formatter.Format(formatter.Source{Text: text}, formatter.DefaultOptions())
		if formatted.Malformed {
			t.Fatalf("malformed ordinary helper %s", name)
		}
		if again := formatter.Format(formatter.Source{Text: formatted.Text}, formatter.DefaultOptions()); again.Text != formatted.Text {
			t.Fatalf("format is not a fixed point: %s", name)
		}
		lex := lexer.NewWithFile(formatted.Text, name)
		for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
			if strings.HasPrefix(token.Lexeme, "__") && token.Type != lexer.IDENT {
				t.Fatalf("private name is not an identifier: %#v", token)
			}
		}
		p := parser.New(lexer.NewWithFile(formatted.Text, name))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		for _, statement := range parsed.Statements {
			if fn, ok := statement.(*ast.FunctionDeclaration); ok {
				if fn.Body == nil || fn.Name.Token.File != name {
					t.Fatalf("body or source owner missing: %#v", fn)
				}
			}
		}
		program.Statements = append(program.Statements, parsed.Statements...)
		program.SourceProvenance[name] = ast.SourceCore
	}
	return program
}

// TestOrdinaryPrivateCoreHelpers covers direct, generic and first-class calls
// in the owner file and module-internal calls from a trusted sibling file.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.2–12.3, 17, 21;
// rules/library/core-library.md — §1.2.
func TestOrdinaryPrivateCoreHelpers(t *testing.T) {
	for _, names := range [][]string{{"helpers.sec", "sibling.sec"}, {"sibling.sec", "helpers.sec"}} {
		a := sema.NewAnalyzer()
		if errs := a.Analyze(load(t, names...)); len(errs) != 0 {
			t.Fatal(errs)
		}
		functions := a.Functions()["__Increment"]
		if len(functions) != 1 || functions[0].CompilerInternal || functions[0].Token.File != "helpers.sec" {
			t.Fatalf("helper lost ordinary declaration identity: %#v", functions)
		}
		for _, file := range []string{"helpers.sec", "./helpers.sec", "sibling.sec", ""} {
			got := sema.FunctionVisibleFromSource(functions[0], file)
			want := file == "helpers.sec" || file == "./helpers.sec"
			if got != want {
				t.Fatalf("source visibility %q = %v, want %v", file, got, want)
			}
		}
	}
}

// TestPrivateCoreHelperDoesNotEscapeFile rejects calls and callable binding
// from other core files; trusted provenance never widens private source scope.
// Rules: rules/foundations/names_scopes_visibility.md — §12.3;
// rules/library/core-library.md — core privilege does not replace visibility.
func TestPrivateCoreHelperDoesNotEscapeFile(t *testing.T) {
	for _, name := range []string{"call_invalid.sec", "value_invalid.sec", "generic_invalid.sec"} {
		t.Run(name, func(t *testing.T) {
			for _, names := range [][]string{{"helpers.sec", name}, {name, "helpers.sec"}} {
				a := sema.NewAnalyzer()
				errs := a.Analyze(load(t, names...))
				if len(errs) == 0 {
					t.Fatal("another trusted core file acquired private access")
				}
				found := false
				for _, err := range errs {
					if strings.Contains(err.Message, "__Increment") || strings.Contains(err.Message, "__Identity") {
						if err.File != name || err.Line == 0 {
							t.Fatalf("missing source location: %#v", err)
						}
						found = true
					}
				}
				if !found {
					t.Fatalf("missing inaccessible helper diagnostic: %v", errs)
				}
			}
		})
	}
}
