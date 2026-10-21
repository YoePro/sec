package semantic

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/sema"
)

// Explicit fallible ranges reuse typed failure routing and the ordinary try
// dispatcher, preserving original endpoints and holder facts on success.
// Rules: rules/errors/errorhandling.md — §§11,15–16,19,35;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,21–23,79–80,131.
func TestPackage16SliceSourceTry(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, _ := sliceBoundaryInput(t, "semantic_ir/slice_source_fallible.sec")
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
				checks, routed, variants, residuals, successes, ends := 0, 0, 0, 0, 0, 0
				var order []string
				for _, block := range fn.Blocks {
					for _, parameter := range block.Parameters {
						typ, _ := m.Types.Lookup(parameter.Type)
						if typ.Kind == TypeSharedSlice || typ.Kind == TypeMutableSlice {
							t.Fatal("single success path introduced a new slice holder")
						}
					}
					for i, op := range block.Operations {
						switch op.Kind {
						case OpSliceRangeCheck:
							checks++
							branch := block.Operations[i+1]
							failure := fn.Blocks[branch.Successors[0].Block]
							if len(failure.Operations) != 1 {
								t.Fatal("range failure did not leave the protected expression immediately")
							}
							forward := failure.Operations[0]
							if forward.Kind != OpBranch || len(forward.Successors) != 1 || len(forward.Successors[0].Arguments) != 1 || forward.Successors[0].Arguments[0] != op.Results[3].ID {
								t.Fatal("failure did not route the check's exact canonical RangeError")
							}
							routed++
							if fn.Name == "Mutable" {
								start, _ := m.Types.Lookup(op.SliceRange.StartType)
								end, _ := m.Types.Lookup(op.SliceRange.EndType)
								if start.Name != "int128" || end.Name != "uint256" || !op.SliceRange.EndExclusive {
									t.Fatal("fallible range narrowed or reinterpreted endpoints")
								}
							}
						case OpBoundsFailure, OpSliceCopyShared, OpSliceMoveMutable, OpArrayExtract, OpStorageLoad:
							t.Fatal("fallible range introduced panic or copied/transferred storage", op.Kind)
						case OpReferenceEndBorrow:
							ends++
						case OpDirectCall:
							order = append(order, string(op.Callee))
						case OpSliceLength:
							order = append(order, "length")
						}
						if op.TryHandlerKind == TryHandlerOK {
							successes++
						}
						if op.Kind == OpEnumConstant && op.TryHandlerKind == TryHandlerErrVariant {
							variants++
						}
						if op.Kind == OpReturn && op.TryHandlerKind == TryHandlerResidual {
							residuals++
						}
					}
				}
				if fn.Name == "Ready" || fn.Name == "First" || fn.Name == "Last" {
					continue
				}
				if checks != 1 || routed != 1 || ends == 0 {
					t.Fatalf("%s: missing fallible range/holder end: %d/%d/%d", fn.Name, checks, routed, ends)
				}
				if fn.Name == "Specific" && (variants != 2 || successes != 1) {
					t.Fatal("specific handlers lost their existing enum dispatch/implicit success metadata")
				}
				if fn.Name == "Partial" && (variants != 1 || residuals != 1 || successes != 1) {
					t.Fatal("guarded partial handler lost residual propagation")
				}
				if fn.Name == "Order" && (len(order) != 3 || !strings.Contains(order[0], "First") || !strings.Contains(order[1], "Last") || order[2] != "length") {
					t.Fatal("endpoint evaluation order/reborrow observation changed", order)
				}
			}
		})
	}
}

// A recovery value needs a holder join, while heterogeneous errors need
// widening. Neither dependency may produce incomplete IR after earlier
// supported functions have already been built.
// Rules: rules/errors/errorhandling.md — §§11,21.1,35;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§22,60–67,83,128.
func TestPackage16SliceTryDependencyBoundaries(t *testing.T) {
	for _, test := range []struct{ file, want string }{
		{"slice_try_recovery.sec", "slice recovery holder joins"},
		{"slice_try_mixed.sec", "heterogeneous failure set"},
	} {
		program, a := sliceBoundaryInput(t, "semantic_ir/"+test.file)
		m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		var unsupported *UnsupportedFeatureError
		if m != nil || !errors.As(err, &unsupported) || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: dependency leaked a module or lost its named boundary: %v", test.file, err)
		}
	}
}

// A successful normalized bound cannot masquerade as the error forwarded
// out of the failed range edge, and reversing the predicate cannot create a
// slice from a failed range even when the error path uses local handlers.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§21–23,79–80,131.
func TestPackage16SliceTryRejectsWrongRangeEdges(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		program, a := sliceBoundaryInput(t, "semantic_ir/slice_source_fallible.sec")
		m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		if err != nil {
			t.Fatal(err)
		}
		changed := false
		for _, fn := range m.Functions {
			if fn.Name != "Fallible" {
				continue
			}
			for _, block := range fn.Blocks {
				for i, op := range block.Operations {
					if op.Kind != OpSliceRangeCheck {
						continue
					}
					branch := &block.Operations[i+1]
					if reverse {
						branch.Successors[0], branch.Successors[1] = branch.Successors[1], branch.Successors[0]
					} else {
						failure := fn.Blocks[branch.Successors[0].Block]
						failure.Operations[0].Successors[0].Arguments[0] = op.Results[0].ID
					}
					changed = true
				}
			}
		}
		if !changed {
			t.Fatal("fixture lost its range check")
		}
		if err := Verify(m); err == nil {
			t.Fatalf("accepted malformed fallible range edge, reversed=%t", reverse)
		}
	}
}
