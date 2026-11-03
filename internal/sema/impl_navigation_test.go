package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Definition navigation joins only validated impl fragments, independent of
// file order, and leaves target identifiers bound to the named type.
// Rules: rules/declarations/impl.md — §§4–5, §22;
// rules/tooling/lsp.md — "One semantic source of truth", "Navigation".
func TestImplNavigation(t *testing.T) {
	for _, order := range [][]string{{"primary", "extension_a", "extension_b"}, {"extension_b", "extension_a", "primary"}} {
		program := &ast.Program{}
		blocks := map[string][]*ast.ImplStatement{}
		for _, name := range order {
			path := "../../testdata/tooling/impl_navigation/" + name + ".sec"
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), path))
			part := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			program.Statements = append(program.Statements, part.Statements...)
			for _, statement := range part.Statements {
				if impl, ok := statement.(*ast.ImplStatement); ok {
					blocks[name] = append(blocks[name], impl)
				}
			}
		}
		a := NewAnalyzer()
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		for i, primary := range blocks["primary"] {
			extensions := []lexer.Token{blocks["extension_a"][i].Token}
			if i == 0 {
				extensions = append(extensions, blocks["extension_b"][0].Token)
			}
			if got := a.DefinitionsAt(primary.Token.File, primary.Token.Line, primary.Token.Column); !reflect.DeepEqual(got, extensions) {
				t.Fatalf("primary links = %v, want %v", got, extensions)
			}
			for _, extension := range extensions {
				if got := a.DefinitionsAt(extension.File, extension.Line, extension.Column); !reflect.DeepEqual(got, []lexer.Token{primary.Token}) {
					t.Fatalf("extension links = %v", got)
				}
			}
			if got := a.DefinitionsAt(primary.Target.Token.File, primary.Target.Token.Line, primary.Target.Token.Column); len(got) != 1 || got[0] == extensions[0] || got[0] == primary.Token {
				t.Fatalf("target identifier lost type definition: %v", got)
			}
		}
	}
}

// Invalid primary/extension headers must not acquire navigation links, and
// a fresh analysis must discard all links from the previous source graph.
// Rules: rules/declarations/impl.md — §§4–5, §27;
// rules/tooling/lsp.md — "Unknown facts".
func TestImplNavigationRejectsInvalidFragmentsAndResets(t *testing.T) {
	program := &ast.Program{}
	var blocks []*ast.ImplStatement
	for _, path := range []string{
		"../../testdata/tooling/impl_navigation/primary.sec",
		"../../testdata/tooling/impl_navigation/extension_a.sec",
		"../../testdata/sema/impl_navigation_invalid/same_module_invalid.sec",
		"../../testdata/sema/impl_navigation_invalid/foreign_invalid.sec",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), path))
		part := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, part.Statements...)
		for _, stmt := range part.Statements {
			if impl, ok := stmt.(*ast.ImplStatement); ok {
				blocks = append(blocks, impl)
			}
		}
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) < 4 {
		t.Fatalf("expected orphan, duplicate, wrong generic and foreign errors: %v", errors)
	}
	invalidCount := 0
	for _, impl := range blocks {
		got := a.DefinitionsAt(impl.Token.File, impl.Token.Line, impl.Token.Column)
		if a.validImplStatements[impl] {
			if len(got) != 1 {
				t.Fatalf("valid block lost navigation: %v", got)
			}
			continue
		}
		invalidCount++
		if len(got) != 0 {
			t.Fatalf("invalid block navigates: %v", got)
		}
	}
	if invalidCount != 4 {
		t.Fatalf("invalid blocks=%d, want 4", invalidCount)
	}
	if errors := a.Analyze(&ast.Program{Statements: program.Statements[:1]}); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, impl := range blocks {
		if got := a.DefinitionsAt(impl.Token.File, impl.Token.Line, impl.Token.Column); len(got) != 0 {
			t.Fatalf("stale navigation after Analyze: %v", got)
		}
	}
}

// A source-organization link is not a production dependency on test code.
// Rules: rules/tooling/testing.md — §§25.1–25.2;
// rules/declarations/impl.md — §5.1.
func TestImplNavigationTestExtensionDoesNotCreateDependency(t *testing.T) {
	program := &ast.Program{}
	var primary, extension *ast.ImplStatement
	for _, name := range []string{"primary", "extension_test"} {
		path := "../../testdata/tooling/impl_navigation/" + name + ".sec"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), path))
		part := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, part.Statements...)
		for _, statement := range part.Statements {
			impl, ok := statement.(*ast.ImplStatement)
			if !ok {
				continue
			}
			if impl.Target.Name == "Widget" {
				if impl.Extends {
					extension = impl
				} else {
					primary = impl
				}
			}
		}
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	got := a.DefinitionsAt(primary.Token.File, primary.Token.Line, primary.Token.Column)
	if !reflect.DeepEqual(got, []lexer.Token{extension.Token}) {
		t.Fatalf("test extension links=%v", got)
	}
	got[0].File = "corrupted"
	if next := a.DefinitionsAt(primary.Token.File, primary.Token.Line, primary.Token.Column); !reflect.DeepEqual(next, []lexer.Token{extension.Token}) {
		t.Fatalf("navigation snapshot changed compiler state: %v", next)
	}
}
