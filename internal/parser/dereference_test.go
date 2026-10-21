package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// MD-063: prefix `*` dereferences; infix `*` stays multiplication. A `*` that
// begins a later line starts a new statement only when it introduces a
// syntactically complete assignment target directly followed by an
// assignment operator; the target's brackets may span lines. The formatter's
// leading-operator multiplication layout keeps its meaning.
func TestParseExplicitDereferencePrecedenceAndStatementStart(t *testing.T) {
	input := `
module main

fn F(r: ref mut int, s: ref Pair) int {
    *r = 5
    *r += 1
    let total :=
          *r
        * 2
    let field := *s.Left
    let product :=
          total
        * factor
    count = 3
    *grid[
        index
    ] = 5
    *r++
    return *r * 2 + total
}
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	var body *ast.BlockStatement
	for _, statement := range program.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok {
			body = function.Body
		}
	}
	if body == nil || len(body.Statements) != 9 {
		t.Fatalf("expected 9 statements, got %#v", body)
	}
	for index, target := range map[int]string{5: "count", 6: "(*grid[index])", 7: "(*r)"} {
		assignment, ok := body.Statements[index].(*ast.AssignmentStatement)
		if !ok || assignment.Target.String() != target {
			t.Fatalf("statement %d = %#v, want assignment to %s", index, body.Statements[index], target)
		}
	}
	for index, operator := range []string{"=", "+="} {
		assignment, ok := body.Statements[index].(*ast.AssignmentStatement)
		if !ok || assignment.Operator != operator || assignment.Target.String() != "(*r)" {
			t.Fatalf("statement %d = %#v, want `*r %s ...`", index, body.Statements[index], operator)
		}
	}
	want := map[int]string{2: "((*r) * 2)", 3: "(*s.Left)", 4: "(total * factor)", 8: "(((*r) * 2) + total)"}
	for index, expected := range want {
		var value ast.Expression
		switch statement := body.Statements[index].(type) {
		case *ast.LetStatement:
			value = statement.Value
		case *ast.ReturnStatement:
			value = statement.Value
		}
		if value == nil || value.String() != expected {
			t.Fatalf("statement %d value = %v, want %s", index, value, expected)
		}
	}
}
