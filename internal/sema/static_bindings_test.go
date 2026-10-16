package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Module-level bindings and function-local `static let` resolve as static
// bindings with stable identities, while ordinary locals and parameters do
// not; every use of a static resolves to the same binding.
//
// Rules:
//   - rules/declarations/static.md — §§ 2, 3, 4
//   - rules/compiler/semantic_ir.md — § 28
func TestResolvedStaticOfDistinguishesStaticStorage(t *testing.T) {
	program := parser.New(lexer.New(`
module main

let limit: int := 7

fn Next(step: int) int {
    static let mut calls: int := 0
    let local := step
    calls = calls + local + limit
    return calls
}
`)).ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	found := map[string]ResolvedStatic{}
	var identifiers []*ast.Identifier
	ast.Visit(program, func(node any) bool {
		if identifier, ok := node.(*ast.Identifier); ok {
			identifiers = append(identifiers, identifier)
		}
		return true
	})
	for _, identifier := range identifiers {
		static, ok := analyzer.ResolvedStaticOf(identifier)
		switch identifier.Value {
		case "limit", "calls":
			if !ok {
				t.Errorf("%s at %d:%d does not resolve as static", identifier.Value, identifier.Token.Line, identifier.Token.Column)
				continue
			}
			if previous, seen := found[identifier.Value]; seen && previous.ID != static.ID {
				t.Errorf("%s resolves to two static identities", identifier.Value)
			}
			found[identifier.Value] = static
		case "local", "step":
			if ok {
				t.Errorf("%s resolves as static", identifier.Value)
			}
		}
	}
	if found["limit"].Function != "" || found["limit"].Mutable || found["calls"].Function != "Next" || !found["calls"].Mutable {
		t.Fatalf("static facts: %+v", found)
	}
	if len(analyzer.ResolvedStaticDeclarations()) != 2 {
		t.Fatalf("want two static declarations, got %+v", analyzer.ResolvedStaticDeclarations())
	}
}
