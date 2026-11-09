package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Module variables are scoped to their module: another module's bodies
// neither see them nor collide with their names, two modules may each
// declare a module variable of the same name, and every body sees its own.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — module scope
func TestModuleVariablesAreModuleScoped(t *testing.T) {
	analyze := func(files map[string]string) []Error {
		program := &ast.Program{}
		for _, name := range []string{"a.sec", "b.sec"} {
			p := parser.New(lexer.NewWithFile(files[name], name))
			parsed := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatalf("%s: %v", name, p.Errors())
			}
			program.Statements = append(program.Statements, parsed.Statements...)
		}
		return NewAnalyzer().Analyze(program)
	}
	errors := analyze(map[string]string{
		"a.sec": "module alpha\n\nlet value := 1\nlet shared := 10\n\nfn A() int {\n    return value + shared\n}\n",
		"b.sec": "module beta\n\nlet shared := 20\n\nfn B() int {\n    let value := 2\n    return value + shared\n}\n",
	})
	if len(errors) != 0 {
		t.Fatalf("module variables leaked across modules: %v", errors)
	}
	errors = analyze(map[string]string{
		"a.sec": "module alpha\n\nlet hidden := 1\n",
		"b.sec": "module beta\n\nfn B() int {\n    return hidden\n}\n",
	})
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "undefined variable hidden") {
		t.Fatalf("another module's variable must not be visible unqualified: %v", errors)
	}
}
