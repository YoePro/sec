package semantic

import (
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// The exact native fixture must build and verify on both scalar plans, with
// all arithmetic/comparison kinds on every primitive floating source type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§20–27,86;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2.
func TestFloatOperatorSourceMatrix(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/build/native/float_operator_matrix.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "float_operator_matrix.sec"))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errs := a.Analyze(parsed.Program); len(errs) != 0 {
			t.Fatal(errs)
		}
		module, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"float_operator_matrix.sec"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		checked := 0
		for _, fn := range module.Functions {
			if fn.Name == "main" {
				continue
			}
			arithmetic := map[FloatBinaryKind]bool{}
			comparisons := map[IntegerComparePredicate]bool{}
			negation := false
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					switch op.Kind {
					case OpFloatNeg:
						negation = true
					case OpFloatBinary:
						arithmetic[op.FloatBinary] = true
					case OpFloatCompare:
						comparisons[op.IntegerCompare] = true
					}
				}
			}
			if !negation || len(arithmetic) != 5 || len(comparisons) != 6 {
				t.Fatalf("plan %d %s incomplete: unary=%v arithmetic=%v comparisons=%v", width, fn.Name, negation, arithmetic, comparisons)
			}
			checked++
		}
		if checked != 3 {
			t.Fatalf("expected three primitive type matrices, got %d", checked)
		}
	}
}
