package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Attributed and ordinary unsafe methods retain the same callable marker and
// declaration/body nodes; neither form becomes an invalid impl member.
// Rules: rules/memory/unsafe.md — §§6–7;
// rules/foundations/grammar.md — §§28,36.
func TestUnsafeImplMethodRetention(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/unsafe_callable_conformance/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.NewWithFile(string(source), "valid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	found := map[string]*ast.FunctionDeclaration{}
	ast.Visit(program, func(node any) bool {
		if _, invalid := node.(*ast.InvalidMember); invalid {
			t.Fatal("unsafe method became invalid member")
		}
		if impl, ok := node.(*ast.ImplStatement); ok {
			for _, member := range impl.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok {
					found[fn.Name.Value] = fn
				}
			}
		}
		return true
	})
	for _, name := range []string{"Read", "Annotated"} {
		fn := found[name]
		if fn == nil || !fn.Unsafe || fn.Extern || fn.Body == nil {
			t.Fatalf("unsafe method %s = %+v", name, fn)
		}
	}
	if found["Wrapper"] == nil || found["Wrapper"].Unsafe || len(found["Annotated"].Attributes) != 1 {
		t.Fatal("method safety/attribute attachment changed")
	}
}
