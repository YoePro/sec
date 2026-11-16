package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestMethodGenericArgumentOwnership retains the independent owner/method
// lists and the indexed-value alternative until Sema resolves the owner.
// Rules: rules/declarations/generics.md — §§9,18,20;
// rules/foundations/grammar.md — generic method calls and member access.
func TestMethodGenericArgumentOwnership(t *testing.T) {
	source, err := os.ReadFile("../../testdata/parser/method_generic_calls.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.NewWithFile(string(source), "method_generic_calls.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	function := program.Statements[1].(*ast.FunctionDeclaration)
	want := []struct {
		owner, method int
		text          string
	}{
		{1, 1, "Box[int].PickStatic[bool](1, true)"},
		{1, 0, "Box[int].PickStatic(1, true)"},
		{0, 1, "value.Pick[bool](1, true)"},
		{1, 1, "values[index].Pick[bool](1, true)"},
	}
	for i, statement := range function.Body.Statements {
		call := statement.(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		member := call.Callee.(*ast.MemberExpression)
		if len(member.OwnerGenericArguments) != want[i].owner || len(call.GenericArguments) != want[i].method || call.String() != want[i].text {
			t.Errorf("call %d: owner=%+v method=%+v text=%s", i, member.OwnerGenericArguments, call.GenericArguments, call.String())
		}
	}
	indexed := function.Body.Statements[3].(*ast.ExpressionStatement).Expression.(*ast.CallExpression).Callee.(*ast.MemberExpression).Object
	if _, ok := indexed.(*ast.IndexExpression); !ok {
		t.Fatal("indexed receiver alternative lost")
	}
}

// TestMalformedGenericOwnerCallRecovery keeps an invalid member node during
// recovery instead of assuming a successful member selector.
// Rules: rules/compiler/parser_recovery.md — missing/skipped token recovery;
// rules/foundations/grammar.md — member access.
func TestMalformedGenericOwnerCallRecovery(t *testing.T) {
	source, err := os.ReadFile("../../testdata/parser/method_generic_calls_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.NewWithFile(string(source), "method_generic_calls_invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("malformed member call accepted")
	}
	found := false
	for _, statement := range program.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name.Value == "Following" {
			found = true
		}
	}
	if !found {
		t.Fatal("member recovery lost following declaration")
	}
}
