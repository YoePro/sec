package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Stray tokens between an if/while condition and a body brace on the same
// line are skipped as one recovery event: the body is parsed as the body, the
// condition becomes an invalid expression, and later impl members stay in the
// impl. Without a brace on that line nothing is consumed (§ 51.2(2)).
// Rules: rules/compiler/parser_recovery.md — §§ 4.5, 51.2, 53.3.
func TestConditionStrayTokensKeepBodyAndEnclosingStructure(t *testing.T) {
	for _, test := range []struct {
		name, source, construct string
	}{
		{"if rune suffix", "module m\ntype S struct {}\nimpl S {\n    fn A(c: rune) int {\n        if c == 'a'r && Check(c) {\n            return 1\n        }\n        return 0\n    }\n    fn B() int {\n        return 2\n    }\n}\n", "if"},
		{"while stray identifier", "module m\ntype S struct {}\nimpl S {\n    fn A(c: int) int {\n        while c == 1 x {\n            return 1\n        }\n        return 0\n    }\n    fn B() int {\n        return 2\n    }\n}\n", "while"},
	} {
		p := New(lexer.NewWithFile(test.source, "m.sec"))
		result := p.Parse()
		if len(p.Errors()) != 1 || len(result.Recovery) == 0 {
			t.Fatalf("%s: errors %v, recovery %v", test.name, p.Errors(), result.Recovery)
		}
		var impl *ast.ImplStatement
		for _, statement := range result.Program.Statements {
			if candidate, ok := statement.(*ast.ImplStatement); ok {
				impl = candidate
			}
			if function, ok := statement.(*ast.FunctionDeclaration); ok {
				t.Fatalf("%s: method %s escaped the impl", test.name, function.Name.Value)
			}
		}
		if impl == nil || len(impl.Members) != 2 {
			t.Fatalf("%s: impl = %+v", test.name, impl)
		}
		body := impl.Members[0].(*ast.FunctionDeclaration).Body.Statements
		switch statement := body[0].(type) {
		case *ast.IfStatement:
			if _, invalid := statement.Condition.(*ast.InvalidExpression); !invalid || statement.Consequence == nil || len(statement.Consequence.Statements) != 1 {
				t.Errorf("%s: if = %#v", test.name, statement)
			}
		case *ast.WhileStatement:
			if _, invalid := statement.Condition.(*ast.InvalidExpression); !invalid || statement.Body == nil || len(statement.Body.Statements) != 1 {
				t.Errorf("%s: while = %#v", test.name, statement)
			}
		default:
			t.Errorf("%s: first statement = %T", test.name, statement)
		}
	}

	// No brace on the condition line: the existing synchronization applies
	// and nothing after the condition line is consumed by this recovery.
	p := New(lexer.NewWithFile("module m\nfn A(c: int) int {\n    if c == 1 x\n    return 0\n}\n", "m.sec"))
	p.Parse()
	if len(p.Errors()) == 0 || p.Errors()[0] != "expected '{' after if condition at 3:15" {
		t.Errorf("no-brace errors = %v", p.Errors())
	}
}
