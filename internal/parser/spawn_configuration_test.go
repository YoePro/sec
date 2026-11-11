package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestNamedThreadConfigurationGrammar keeps the config and child callable as
// distinct AST identities while unconfigured task/thread expressions stay intact.
// Rules: rules/concurrency/spawn.md — §4(1),(10); threads.md — §10(3).
func TestNamedThreadConfigurationGrammar(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_named_config_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	configured, plain := 0, 0
	ast.Visit(program, func(node any) bool {
		spawn, ok := node.(*ast.SpawnExpression)
		if !ok {
			return true
		}
		if _, ok := spawn.Value.(*ast.CallExpression); !ok {
			t.Fatal(spawn)
		}
		if spawn.Configuration != nil {
			configured++
			move, ok := spawn.Configuration.(*ast.PrefixExpression)
			if !ok || move.Operator != "<-" || spawn.Kind != "thread" {
				t.Fatal(spawn)
			}
		} else {
			plain++
		}
		return true
	})
	if configured != 3 || plain != 2 {
		t.Fatal(configured, plain)
	}
}

// TestNamedThreadConfigurationRecovery preserves following syntax after an
// owning configuration lacks its required child callable.
// Rules: rules/concurrency/threads.md — §10(3); rules/tooling/diagnostics.md — recovery.
func TestNamedThreadConfigurationRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/thread_config_missing_callable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 1 || len(program.Statements) != 3 {
		t.Fatal(p.Errors(), program)
	}
	function := program.Statements[1].(*ast.FunctionDeclaration)
	if len(function.Body.Statements) != 3 {
		t.Fatal(function.Body)
	}
	if program.Statements[2].(*ast.FunctionDeclaration).Name.Value != "After" {
		t.Fatal(program)
	}
}
