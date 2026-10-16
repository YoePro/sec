package semantic

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestShapedMultiplyConsumerFacts verifies that IR clients obtain the existing
// Sema decision without deriving shape/scalar algebra again. Shaped value
// representation remains an explicit unsupported boundary, not a scalar cast.
// Rules: rules/collections/shaped-types.md — §§22,29,35;
// rules/compiler/semantic_ir.md — §91 "Semantic plans versus AST".
func TestShapedMultiplyConsumerFacts(t *testing.T) {
	const file = "../../../testdata/sema/shaped_multiply_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	fn := program.Statements[1].(*ast.FunctionDeclaration)
	expr := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.InfixExpression)
	plan, found := a.ResolvedShapedMultiplyOf(expr)
	if !found || plan.ResultType.StaticElementCount.String() != "8" || plan.Inner != 3 || plan.ProductElement.FloatBits != 32 {
		t.Fatal(plan)
	}
	if module, err := Build(program, a, BuildOptions{RequestedModule: "main"}); err == nil || module != nil {
		t.Fatal("shaped representation boundary lost", module, err)
	}
}
