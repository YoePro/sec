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

// TestInlineThreadConfigurationGrammar distinguishes contextual fields and
// empty inline configuration from ordinary spawn statement blocks. Recovery
// retains the next binding and declaration after a missing child callable.
// Rules: rules/concurrency/threads.md — §11;
// rules/concurrency/spawn.md — §4; rules/compiler/parser_recovery.md — expression recovery.
func TestInlineThreadConfigurationGrammar(t *testing.T) {
	for _, tc := range []struct {
		file                       string
		errors, configured, bodies int
	}{
		{"../../testdata/sema/thread_config_valid.sec", 0, 3, 0},
		{"../../testdata/parser/thread_inline_config_blocks.sec", 0, 2, 3},
		{"../../testdata/parser/thread_inline_config_recovery.sec", 1, 1, 0},
	} {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		p := New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != tc.errors {
			t.Fatal(tc.file, p.Errors())
		}
		configured, bodies := 0, 0
		ast.Visit(program, func(node any) bool {
			if spawn, ok := node.(*ast.SpawnExpression); ok {
				if spawn.Body != nil {
					bodies++
				}
				if spawn.InlineConfiguration {
					configured++
					literal, ok := spawn.Configuration.(*ast.StructLiteral)
					if !ok || literal.Type.Name != "ThreadConfig" || spawn.Value == nil {
						t.Fatal(spawn)
					}
				}
			}
			return true
		})
		if configured != tc.configured || bodies != tc.bodies {
			t.Fatal(tc.file, configured, bodies)
		}
		if tc.errors != 0 {
			function := program.Statements[1].(*ast.FunctionDeclaration)
			if len(function.Body.Statements) != 3 || program.Statements[2].(*ast.FunctionDeclaration).Name.Value != "After" {
				t.Fatal(program)
			}
		}
	}
}
