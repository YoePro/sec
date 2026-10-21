package semantic

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/sema"
)

// Source construction preserves all range forms, exact target domains,
// writable versus shared Places, empty provenance and success-only borrows.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,20,42,78–80,130.
func TestPackage16FixedSliceSource(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, _ := sliceBoundaryInput(t, "semantic_ir/slice_source_fixed.sec")
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
				var calls []string
				borrows, normalizes, checks, failures, ends := 0, 0, 0, 0, 0
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						switch op.Kind {
						case OpSliceBorrowShared, OpSliceBorrowMutable:
							borrows++
						case OpSliceRangeNormalize:
							normalizes++
						case OpSliceRangeCheck:
							checks++
							if fn.Name == "Dynamic" {
								start, _ := m.Types.Lookup(op.SliceRange.StartType)
								end, _ := m.Types.Lookup(op.SliceRange.EndType)
								if start.Name != "int128" || end.Name != "uint256" || !op.SliceRange.StartSigned || op.SliceRange.EndSigned || op.SliceRange.EndExclusive {
									t.Fatal("range check lost original endpoints")
								}
							}
						case OpReferenceEndBorrow:
							ends++
						case OpBoundsFailure:
							if op.ArrayOperation == "slice-range" {
								failures++
							}
						case OpDirectCall:
							calls = append(calls, string(op.Callee))
						case OpStorageLoad, OpArrayExtract, OpArrayReplace:
							t.Fatalf("%s copied aggregate while slicing", fn.Name)
						}
					}
				}
				if ends != borrows {
					t.Fatalf("%s: discarded slice holders=%d borrows=%d", fn.Name, ends, borrows)
				}
				if fn.Name == "Forms" && (borrows != 7 || normalizes != 7 || checks != 0 || failures != 0) {
					t.Fatal("range forms lost", borrows, normalizes, checks, failures)
				}
				if fn.Name == "Dynamic" && (borrows != 1 || checks != 1 || failures != 1) {
					t.Fatal("missing ordinary runtime CFG")
				}
				if fn.Name == "Order" && (len(calls) != 3 || !strings.Contains(calls[0], "Row") || !strings.Contains(calls[1], "First") || !strings.Contains(calls[2], "Last")) {
					t.Fatal("source/bounds evaluation order", calls)
				}
			}
		})
	}
}

// Ordinary range provenance and guard direction are verified even after the
// original source has been lowered. Fallible ranges use typed try edges.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§20–23,80,128–131.
func TestPackage16SourceSliceFailureBoundaries(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/slice_source_fallible.sec")
	m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		program, a = sliceBoundaryInput(t, "semantic_ir/slice_source_fixed.sec")
		m, err = Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		if err != nil {
			t.Fatal(err)
		}
		for _, fn := range m.Functions {
			if fn.Name != "Dynamic" {
				continue
			}
			if reverse {
				branch := &fn.Blocks[0].Operations[len(fn.Blocks[0].Operations)-1]
				branch.Successors[0], branch.Successors[1] = branch.Successors[1], branch.Successors[0]
			} else {
				for _, block := range fn.Blocks {
					if len(block.Operations) == 1 && block.Operations[0].Kind == OpBoundsFailure {
						block.Operations[0].ArrayOperation = "slice-index"
					}
				}
			}
		}
		if err := Verify(m); err == nil {
			t.Fatalf("accepted malformed ordinary range failure, reversed=%v", reverse)
		}
	}
}
