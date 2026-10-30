package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestRegisterOperationLookupPreservesQualifiedFunctions models the loader's
// qualified declaration names. Register operation probing must not evaluate a
// namespace as a runtime receiver merely because Read/Write/IsShadowKnown match.
// Rules: rules/compiler/compiler_known_members.md — "Lookup order";
// rules/declarations/registers.md — §11.7, reserved register operations.
func TestRegisterOperationLookupPreservesQualifiedFunctions(t *testing.T) {
	const file = "../../testdata/sema/register_operation_namespace.sec"
	input, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(input), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	for _, statement := range program.Statements {
		if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Value != "Run" {
			fn.Name.Value = "stream." + fn.Name.Value
		}
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("qualified free functions treated as register receivers: %v", errors)
	}
}
