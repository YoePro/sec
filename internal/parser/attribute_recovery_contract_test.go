package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Malformed compiler-known attribute arguments keep the attribute, its parsed
// and malformed argument positions, adjacent attributes, the owning
// declaration and later siblings, with one causal diagnostic; a stray closing
// delimiter after a complete attribute does not detach it.
// Rules: rules/compiler/parser_recovery.md — §§ 27(1)–(3), 82.2, 82.3.
func TestAttributeRecoveryContract(t *testing.T) {
	for _, test := range []struct {
		name, source string
		attributes   int
		arguments    []string
	}{
		{"missing argument value", "@target(os: linux, arch: , abi: gnu)\n@noPanic\nfn A() void {}\nfn B() void {}\n", 2, []string{"os", "arch", "abi"}},
		{"reserved token argument", "@target(os: linux, ???)\nfn A() void {}\nfn B() void {}\n", 1, nil},
		{"incomplete expression", "@when(debug and )\nfn A() void {}\nfn B() void {}\n", 1, nil},
		{"missing closing parenthesis", "@target(os: linux\n@noPanic\nfn A() void {}\nfn B() void {}\n", 2, nil},
		{"stray closing parenthesis", "@target(os: linux, arch: amd64))\nfn A() void {}\nfn B() void {}\n", 1, []string{"os", "arch"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := New(lexer.NewWithFile("module main\n"+test.source, "attributes.sec")).Parse()
			if len(result.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %v, want one causal diagnostic", result.Diagnostics)
			}
			functions := map[string]*ast.FunctionDeclaration{}
			for _, statement := range result.Program.Statements {
				if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name != nil {
					functions[fn.Name.Value] = fn
				}
			}
			a, b := functions["A"], functions["B"]
			if a == nil || b == nil {
				t.Fatalf("lost a declaration: %v", functions)
			}
			if len(a.Attributes) != test.attributes {
				t.Fatalf("A has %d attributes, want %d", len(a.Attributes), test.attributes)
			}
			if test.arguments != nil {
				var names []string
				for _, argument := range a.Attributes[0].Arguments {
					if argument.Name != nil {
						names = append(names, argument.Name.Value)
					}
				}
				if len(names) != len(test.arguments) {
					t.Fatalf("arguments = %v, want %v", names, test.arguments)
				}
				for index := range names {
					if names[index] != test.arguments[index] {
						t.Fatalf("arguments = %v, want %v", names, test.arguments)
					}
				}
			}
		})
	}
}
