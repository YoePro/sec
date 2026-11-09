package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestSpawnGrammarForms preserves expression/block operands and explicit kind
// tokens, while contextual kind words remain ordinary identifiers elsewhere.
// Rules: rules/foundations/grammar.md — §89;
// rules/concurrency/spawn.md — §2(1–9).
func TestSpawnGrammarForms(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/spawn_forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	function := program.Statements[2].(*ast.FunctionDeclaration)
	count := 0
	for _, statement := range function.Body.Statements {
		binding, ok := statement.(*ast.LetStatement)
		if !ok {
			continue
		}
		spawn, ok := binding.Value.(*ast.SpawnExpression)
		if !ok {
			continue
		}
		want := []string{"task", "task", "thread", "process"}[count%4]
		if spawn.Kind != want || (spawn.KindToken.Lexeme != "") != (count%4 != 0) || (spawn.Body != nil) != (count >= 4) || (spawn.Value != nil) != (count < 4) {
			t.Fatalf("form %d: %+v", count, spawn)
		}
		if count < 4 {
			if _, ok := spawn.Value.(*ast.CallExpression); !ok {
				t.Fatalf("operand %T", spawn.Value)
			}
		}
		count++
	}
	if count != 8 {
		t.Fatal(count)
	}
}

// TestSpawnMissingOperandRecovery keeps the following binding, closing block
// and function intact after missing operands in every execution-kind form.
// Rules: rules/foundations/grammar.md — §89;
// rules/tooling/diagnostics.md — parser recovery.
func TestSpawnMissingOperandRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/spawn_missing_operand_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 4 {
		t.Fatal(p.Errors())
	}
	if len(program.Statements) != 3 {
		t.Fatal(program.String())
	}
	function := program.Statements[1].(*ast.FunctionDeclaration)
	if len(function.Body.Statements) != 4 {
		t.Fatal(function.Body)
	}
	if after, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || after.Name.Value != "After" {
		t.Fatal(program.String())
	}
}
