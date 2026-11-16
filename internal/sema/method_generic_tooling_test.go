package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMethodTypeArgumentFacts publishes independent ordered environments without
// exposing analyzer-owned slices or type records to tooling consumers.
// Rules: rules/declarations/generics.md — §§9,18–20,30;
// rules/tooling/lsp.md — §12(1),(3).
func TestMethodTypeArgumentFacts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/generics/method_hover.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "method_hover.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	count := 0
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		call := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
		fact, ok := a.ResolvedMethodTypeArgumentsOf(call)
		if !ok || len(fact.Owner) != 1 || len(fact.Method) != 1 || fact.Owner[0].Name != "T" || fact.Method[0].Name != "U" || fact.Identity == "" {
			t.Fatalf("missing bindings: %+v", fact)
		}
		before, _ := a.ResolvedMethodTypeArgumentsOf(call)
		fact.Owner[0].Type.Name = "changed"
		fact.Method[0].Type.Name = "changed"
		after, _ := a.ResolvedMethodTypeArgumentsOf(call)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("tooling mutated compiler facts")
		}
		count++
	}
	if count != 4 {
		t.Fatal("calls", count)
	}
	if _, ok := a.ResolvedMethodTypeArgumentsOf(nil); ok {
		t.Fatal("unresolved call acquired bindings")
	}
	a.Analyze(&ast.Program{})
	for _, statement := range program.Statements {
		if fn, ok := statement.(*ast.FunctionDeclaration); ok {
			call := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
			if _, ok := a.ResolvedMethodTypeArgumentsOf(call); ok {
				t.Fatal("stale call facts")
			}
		}
	}
}
