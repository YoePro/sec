package parser

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestCompilationUnitFileAttributes checks that file attachment survives
// multi-file assembly, remains distinct from statement attachment, and retains
// the actual source nodes rather than synthesized compatibility attributes.
// Rules: rules/foundations/attributes.md — §§ 15(1–6), 16(1), 17, 25(2–5).
func TestCompilationUnitFileAttributes(t *testing.T) {
	combined := &ast.Program{}
	var originals []*ast.TargetDirective
	for _, name := range []string{"canonical.sec", "legacy.sec"} {
		source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "parser", "file_attributes", name))
		if err != nil {
			t.Fatal(err)
		}
		p := New(lexer.NewWithFile(string(source), name))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		directives := parsed.FileTargetDirectives()
		if len(directives) != 1 {
			t.Fatalf("%s: file directives = %v", name, directives)
		}
		originals = append(originals, directives[0])
		combined.Statements = append(combined.Statements, parsed.Statements...)
	}
	directives := combined.FileTargetDirectives()
	if len(directives) != 2 || directives[0] != originals[0] || directives[1] != originals[1] {
		t.Fatalf("assembly lost source identity/order: %v", directives)
	}
	canonical, legacy := directives[0], directives[1]
	if !canonical.Canonical || canonical.Attribute == nil || len(canonical.Selectors) != 5 || canonical.Token.File != "canonical.sec" {
		t.Fatalf("canonical file attribute = %+v", canonical)
	}
	if canonical.Attribute.Token != canonical.Token || canonical.Attribute.Name.Value != "target" || len(canonical.Attribute.Arguments) != 5 {
		t.Fatalf("written attribute was not retained: %+v", canonical.Attribute)
	}
	if legacy.Canonical || legacy.Attribute != nil || len(legacy.Selectors) != 2 || legacy.Token.File != "legacy.sec" {
		t.Fatalf("legacy directive changed form: %+v", legacy)
	}
	var selected *ast.FunctionDeclaration
	visits := map[*ast.Attribute]int{}
	ast.Visit(combined, func(node any) bool {
		if attr, ok := node.(*ast.Attribute); ok {
			visits[attr]++
		}
		if fn, ok := node.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Selected" {
			selected = fn
		}
		return true
	})
	if selected == nil || len(selected.Attributes) != 1 || selected.Attributes[0] == canonical.Attribute {
		t.Fatalf("file and statement attachments mixed: %+v", selected)
	}
	if len(visits) != 2 || visits[canonical.Attribute] != 1 || visits[selected.Attributes[0]] != 1 {
		t.Fatalf("source attributes must be traversed once: %v", visits)
	}
	// Filtering or assembling a new unit must not retain stale file metadata.
	filtered := &ast.Program{Statements: []ast.Statement{selected}}
	if len(filtered.FileTargetDirectives()) != 0 {
		t.Fatal("statement attribute became a file attribute")
	}
	directives[0] = nil
	if combined.FileTargetDirectives()[0] != canonical {
		t.Fatal("returned slice aliases the unit's storage")
	}
	var absent *ast.Program
	if len(absent.FileTargetDirectives()) != 0 {
		t.Fatal("nil program has file attributes")
	}
}
