package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

func mutFnImplDiagnostics(t *testing.T, source string) ([]Diagnostic, *ast.Program) {
	t.Helper()
	p := New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	return p.Diagnostics(), parsed.Program
}

// `mut fn` on an impl method is its own error at the `mut` token, also with
// attributes and in impl extends; the method is still parsed so that it is
// checked and no missing-member error follows.
// Rules: rules/declarations/impl.md — receiver derivation.
func TestMutFnInImplIsDedicatedError(t *testing.T) {
	for _, test := range []struct {
		name, source string
		line, column int
	}{
		{"primary", "module main\ntype A struct { X: int }\nimpl A {\n    mut fn Set() void { self.X = 1 }\n}\n", 4, 5},
		{"implements", "module main\ntype A struct { X: int }\nimpl A implements Reset {\n    mut fn Reset() void { self.X = 1 }\n}\n", 4, 5},
		{"extends", "module main\ntype A struct { X: int }\nimpl extends A {\n    mut fn Set() void { self.X = 1 }\n}\n", 4, 5},
		{"attributed", "module main\ntype A struct { X: int }\nimpl A {\n    @noPanic\n    mut fn Set() void { self.X = 1 }\n}\n", 5, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics, program := mutFnImplDiagnostics(t, test.source)
			if len(diagnostics) != 1 || diagnostics[0].ID != compilerdiagnostics.ParserMutFnInImpl {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
			got := diagnostics[0]
			if got.Primary.Line != test.line || got.Primary.Column != test.column || !strings.Contains(got.Help, "Write `fn`") {
				t.Fatalf("diagnostic = %#v", got)
			}
			var methods []string
			for _, statement := range program.Statements {
				if impl, ok := statement.(*ast.ImplStatement); ok {
					for _, member := range impl.Members {
						if fn, ok := member.(*ast.FunctionDeclaration); ok {
							methods = append(methods, fn.Name.Value)
						}
					}
				}
			}
			if len(methods) != 1 {
				t.Fatalf("method not retained: %v", methods)
			}
		})
	}
}

// The canonical forms stay valid: `mut fn` in an interface and as a callable
// type, plain `fn` in an impl; a stray `mut` member is not reported as P2037.
func TestMutFnOutsideImplMethodsIsUnchanged(t *testing.T) {
	for _, source := range []string{
		"module main\ninterface Reset {\n    mut fn Reset() void\n}\n",
		"module main\ntype A struct { X: int }\nimpl A {\n    fn Set() void { self.X = 1 }\n}\n",
		"module main\nfn Apply(action: mut fn() int) int { return action() }\n",
	} {
		if diagnostics, _ := mutFnImplDiagnostics(t, source); len(diagnostics) != 0 {
			t.Fatalf("%s: %#v", source, diagnostics)
		}
	}
	diagnostics, _ := mutFnImplDiagnostics(t, "module main\ntype A struct { X: int }\nimpl A {\n    mut let x := 1\n}\n")
	for _, diagnostic := range diagnostics {
		if diagnostic.ID == compilerdiagnostics.ParserMutFnInImpl {
			t.Fatalf("stray mut reported as mut fn: %#v", diagnostic)
		}
	}
}
