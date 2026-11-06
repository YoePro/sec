package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// All static/instance categories share the primary/extension namespace, in
// either source order. Conflicting storage never enters the symbol registry.
// Rules: rules/declarations/static.md — §6; rules/declarations/impl.md — §23.
func TestImplStorageNamespace(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/impl_storage_namespace/conflicts_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 54 {
		for _, e := range errors {
			t.Logf("%+v", e)
		}
		t.Fatalf("errors=%d want54", len(errors))
	}
	for _, e := range errors {
		if e.ID != diagnostics.AssociatedStorageConflict || e.Help == "" || e.PreviousLine == 0 || e.Line == e.PreviousLine && e.Column == e.PreviousColumn {
			t.Fatalf("incomplete conflict: %+v", e)
		}
	}
	for name := range a.symbols.all() {
		if len(name) > 7 && name[len(name)-7:] == ".Shared" {
			t.Fatalf("invalid storage published: %s", name)
		}
	}
}

// Separate source files contribute to the same namespace regardless of visitation
// order, and diagnostics retain the actual source file for each declaration.
// Rules: rules/declarations/impl.md — §§22–23;
// rules/declarations/static.md — §6.
func TestImplStorageNamespaceFiles(t *testing.T) {
	for _, order := range [][]string{{"primary", "extension"}, {"extension", "primary"}} {
		program := &ast.Program{}
		for _, name := range order {
			path := "../../testdata/sema/impl_storage_namespace/" + name + ".sec"
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
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if len(errors) != 1 || errors[0].ID != diagnostics.AssociatedStorageConflict || errors[0].File != "../../testdata/sema/impl_storage_namespace/primary.sec" || errors[0].PreviousFile != "../../testdata/sema/impl_storage_namespace/extension.sec" {
			t.Fatal(errors)
		}
	}
	data, err := os.ReadFile("../../testdata/sema/impl_storage_namespace/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, name := range []string{"Counter.Shared", "Counter.Additional", "Other.Shared", "Box.Shared"} {
		if _, ok := a.symbols.lookup(name); !ok {
			t.Errorf("lost valid associated binding %s", name)
		}
	}
}
