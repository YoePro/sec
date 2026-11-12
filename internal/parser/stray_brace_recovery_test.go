package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// A deeper-indented extra `}` followed by another case keeps the switch open,
// and one followed by a statement keeps the function open; each is reported
// once and every later case and impl member keeps its place (MD-132).
// Valid programs with unusual indentation parse unchanged.
// Rules: rules/compiler/parser_recovery.md — § 24 (MD-132).
func TestStrayClosingBraceRecovery(t *testing.T) {
	switchSource := `module m
type S struct {}
impl S {
    fn Kind(c: int) int {
        switch c {
            case 1:
                if c == 1 {
                    return 10
                }
                return 11
            }
            case 2:
                return 20
            default:
                return 0
        }
    }
    fn After() int {
        return 1
    }
}
`
	p := New(lexer.NewWithFile(switchSource, "m.sec"))
	program := p.Parse().Program
	if len(p.Errors()) != 1 || !strings.Contains(p.Errors()[0], "unexpected '}' at 11:13: the switch opened at 5:9 continues with \"case\"") {
		t.Fatalf("switch errors = %v", p.Errors())
	}
	impl := implOf(t, program)
	if len(impl.Members) != 2 {
		t.Fatalf("impl members = %d", len(impl.Members))
	}
	switchStatement := impl.Members[0].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.SwitchStatement)
	if len(switchStatement.Cases) != 2 || switchStatement.Default == nil {
		t.Fatalf("switch cases = %d default = %v", len(switchStatement.Cases), switchStatement.Default != nil)
	}

	functionSource := `module m
type S struct {}
impl S {
    fn Count(c: int) int {
        let mut total := 0
            }
        if c > 0 {
            total = c
        }
        return total
    }
    fn After() int {
        return 1
    }
}
`
	p = New(lexer.NewWithFile(functionSource, "m.sec"))
	program = p.Parse().Program
	if len(p.Errors()) != 1 || !strings.Contains(p.Errors()[0], "the function body opened at 4:26 continues with \"if\"") {
		t.Fatalf("function errors = %v", p.Errors())
	}
	if impl := implOf(t, program); len(impl.Members) != 2 || len(impl.Members[0].(*ast.FunctionDeclaration).Body.Statements) != 3 {
		t.Fatalf("function impl = %+v", impl)
	}

	// Valid programs are never reinterpreted: a misindented but correct
	// closing brace followed by ordinary code still closes its construct.
	for _, valid := range []string{
		"module m\nfn A(c: int) int {\n    if c > 0 {\n        return 1\n        }\n        return 0\n}\n",
		"module m\nfn A(c: int) int {\n    switch c {\n        case 1:\n            return 1\n            }\n    return 0\n}\n",
		"module m\nfn A() int {\n    return 1\n    }\nfn B() int {\n    return 2\n}\n",
	} {
		p := New(lexer.NewWithFile(valid, "m.sec"))
		p.Parse()
		if len(p.Errors()) != 0 {
			t.Errorf("valid source reported %v:\n%s", p.Errors(), valid)
		}
	}
}

func implOf(t *testing.T, program *ast.Program) *ast.ImplStatement {
	t.Helper()
	for _, statement := range program.Statements {
		if impl, ok := statement.(*ast.ImplStatement); ok {
			return impl
		}
		if function, ok := statement.(*ast.FunctionDeclaration); ok {
			t.Fatalf("method %s escaped the impl", function.Name.Value)
		}
	}
	t.Fatal("no impl")
	return nil
}
