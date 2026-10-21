package semantic

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/sema"
)

// Source reborrows retain source SSA, narrowed ranges and P15 facts without
// transferring a holder or loading/copying elements. Runtime endpoints keep
// original carriers and calls occur once before the spatial observation.
// Rules: rules/compiler/semantic_ir.md — §126.8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,43–46,60–62.
func TestPackage16SliceSourceReborrow(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, _ := sliceBoundaryInput(t, "semantic_ir/slice_source_reborrow.sec")
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			for _, fn := range m.Functions {
				shared, mutable, checks, lengths := 0, 0, 0, 0
				var order []string
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						switch op.Kind {
						case OpSliceReborrowShared:
							shared++
						case OpSliceReborrowMutable:
							mutable++
						case OpSliceRangeCheck:
							checks++
						case OpSliceLength:
							lengths++
							order = append(order, "length")
						case OpDirectCall:
							order = append(order, string(op.Callee))
						case OpSliceCopyShared, OpSliceMoveMutable, OpArrayExtract, OpStorageLoad:
							t.Fatal("reborrow transferred holder or backing elements", op.Kind)
						}
					}
				}
				if fn.Name == "Full" && (shared != 2 || mutable != 0 || checks != 0 || lengths != 2) {
					t.Fatal("full/nested proven reborrows", shared, mutable, checks, lengths)
				}
				if fn.Name == "Mutable" && (shared != 1 || mutable != 1) {
					t.Fatal("lost mutable/shared authorities")
				}
				if fn.Name == "Dynamic" && (shared != 1 || checks != 1) {
					t.Fatal("lost dynamic range check")
				}
				if fn.Name == "Order" && (len(order) != 3 || !strings.Contains(order[0], "First") || !strings.Contains(order[1], "Last") || order[2] != "length") {
					t.Fatal("endpoint/observation order", order)
				}
			}
		})
	}
}
