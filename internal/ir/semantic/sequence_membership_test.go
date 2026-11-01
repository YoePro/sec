package semantic

import (
	"os"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"testing"
)

// Scalar membership preserves equality, order, source reuse and bounded IR
// size across fixed-array and P16 slice representations on both target plans.
// Rules: rules/foundations/operators.md — §A.14;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45–58.
func TestSequenceMembershipScalarProducer(t *testing.T) {
	const path = "../../../testdata/semantic_ir/sequence_membership/scalars.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if errors := p.Errors(); len(errors) != 0 {
			t.Fatal(errors)
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		module, err := Build(program, a, BuildOptions{MaxPackage: 16})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err, Format(module))
		}
		names := map[FunctionID]string{}
		for _, fn := range module.Functions {
			names[fn.ID] = fn.Name
		}
		for _, fn := range module.Functions {
			for _, block := range fn.Blocks {
				for i := range block.Operations {
					op := &block.Operations[i]
					if op.Kind == OpFloatCompare && op.IntegerCompare != IntegerCompareEQ {
						t.Fatal("membership changed NaN equality", *op)
					}
					if (op.Kind == OpArrayExtract || op.Kind == OpSliceElementPlace) && op.ArrayCheckKind == ArrayIndexRuntimeCheck {
						guard := op.ArrayGuard
						op.ArrayGuard = 0
						if err := Verify(module); err == nil {
							t.Fatal("unguarded membership read accepted")
						}
						op.ArrayGuard = guard
					}
				}
			}
			if fn.Name == "Effects" || fn.Name == "SliceEffects" {
				calls := []string{}
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						if op.Kind == OpDirectCall {
							calls = append(calls, names[op.Callee])
						}
					}
				}
				second := "Source"
				if fn.Name == "SliceEffects" {
					second = "View"
				}
				if len(calls) != 2 || calls[0] != "Needle" || calls[1] != second {
					t.Fatalf("%s duplicates effectful operands: %v", fn.Name, calls)
				}
			}
			if fn.Name == "Long" || fn.Name == "Large" {
				extracts := 0
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						if op.Kind == OpArrayExtract {
							extracts++
						}
					}
				}
				if extracts != 1 || len(fn.Blocks) > 16 {
					t.Fatalf("%s unrolled search: %d extracts / %d blocks", fn.Name, extracts, len(fn.Blocks))
				}
			}
		}
	}
}
