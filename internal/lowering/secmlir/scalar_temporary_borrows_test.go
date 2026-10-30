package secmlir

import (
	"os"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"testing"
)

// Scalar literal/operator loans use ordinary P15 temporary Places and pass the
// same native guards as constructor temporaries on both widths/schema versions.
// Rules: rules/memory/borrowing.md §5.4; rules/mlir/packages/sec-mlir-dialect_package15.md §§59–60.
func TestScalarTemporaryBorrowsEmit(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/scalar_temporary_borrows.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "scalar-temporary.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
		if errors := a.Analyze(program); len(errors) > 0 {
			t.Fatal(errors)
		}
		module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 15})
		if err != nil {
			t.Fatal(err)
		}
		for _, schema := range []uint32{11, 12} {
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: schema})
			if err != nil {
				t.Fatal(err)
			}
			verifyPackage15MLIR(t, "scalar-temporary", output)
		}
	}
}
