package semantic

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/sema"
)

// The supported source matrix is built directly from analyzed source on both
// target plans, without patching generated IR or installing white-box facts.
// It relates source plans to emitted access/check counts, rejects aggregate
// copies and verifies repeat-build determinism and predecessor boundaries.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,33–37,128–137,141.
func TestPackage16SourceMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		covered := map[OpKind]bool{}
		for _, file := range []string{
			"slice_source_fixed.sec", "slice_source_reborrow.sec",
			"slice_source_length.sec", "slice_source_fallible.sec",
			"slice_source_index.sec", "../sema/slice_plan_queries_valid.sec",
		} {
			t.Run(fmt.Sprintf("%d/%s", width, file), func(t *testing.T) {
				program := sliceBoundaryProgram(t, "semantic_ir/"+file)
				a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				if errs := a.Analyze(program); len(errs) != 0 {
					t.Fatal(errs)
				}
				expected := map[OpKind]int{}
				if err := walk.Inspect(program, func(node any) error {
					switch expr := node.(type) {
					case *ast.RefExpression:
						if plan, ok := a.ResolvedSliceCreationPlanOf(expr); ok {
							check := OpSliceRangeNormalize
							if plan.CheckKind == sema.SliceRangeRuntimeCheck {
								check = OpSliceRangeCheck
							}
							expected[check]++
							if plan.SourceKind == sema.SliceSourceSlice {
								expected[OpSliceLength]++
							}
						}
					case *ast.IndexExpression:
						if plan, ok := a.ResolvedSliceIndexPlanOf(expr); ok {
							expected[OpSliceElementPlace]++
							if plan.CheckKind == sema.ArrayIndexRuntimeCheck {
								expected[OpSliceIndexInBounds]++
							}
						}
					}
					if expr, ok := node.(ast.Expression); ok {
						if _, ok := a.ResolvedSliceLengthPlanOf(expr); ok {
							expected[OpSliceLength]++
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
				if err != nil {
					t.Fatal(err)
				}
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				if m.PointerWidthBits != width {
					t.Fatal("source matrix lost its resolved target plan")
				}
				actual := map[OpKind]int{}
				for _, fn := range m.Functions {
					for _, block := range fn.Blocks {
						for _, op := range block.Operations {
							actual[op.Kind]++
							covered[op.Kind] = true
							if strings.HasPrefix(string(op.Kind), "slice.") && (op.Location.File == "" || op.Location.Line == 0) {
								t.Fatal("source slice operation lost its source location", op.Kind)
							}
							if op.Kind == OpArrayExtract || op.Kind == OpArrayReplace || op.Kind == OpStorageLoad || op.Kind == OpSliceCopyShared || op.Kind == OpSliceMoveMutable {
								t.Fatal("source slicing copied backing elements or implicitly transferred a holder", op.Kind)
							}
						}
					}
				}
				for _, kind := range []OpKind{OpSliceRangeNormalize, OpSliceRangeCheck, OpSliceLength, OpSliceElementPlace, OpSliceIndexInBounds} {
					if actual[kind] != expected[kind] {
						t.Fatalf("%s: source plans=%d emitted=%d (omitted or redundant operation)", kind, expected[kind], actual[kind])
					}
				}
				again, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
				if err != nil {
					t.Fatal(err)
				}
				if Format(m) != Format(again) {
					t.Fatal("repeat source build changed IR or semantic identities")
				}
			})
		}
		for _, kind := range []OpKind{OpSliceBorrowShared, OpSliceBorrowMutable, OpSliceReborrowShared, OpSliceReborrowMutable, OpSliceRangeNormalize, OpSliceRangeCheck, OpSliceLength, OpSliceIndexInBounds, OpSliceElementPlace, OpPlaceRead, OpPlaceWrite, OpReferenceBorrowShared, OpReferenceBorrowMutable, OpReferenceEndBorrow, OpResultErr} {
			if !covered[kind] {
				t.Errorf("%d-bit source matrix lost %s coverage", width, kind)
			}
		}
	}
}

// Every currently deferred source dependency is distinguished from a Sema
// error and fails without a module. A supported preceding function cannot
// turn such a failure into partial success; default/P15 tracks stay separate.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§115–116,128–137;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestPackage16SourceDependencyMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, test := range []struct{ file, want string }{
			{"package16_boundary_owner.sec", "dynamic"},
			{"slice_length_forms.sec", "P16-36"},
			{"slice_try_recovery.sec", "slice recovery holder joins"},
			{"slice_try_mixed.sec", "heterogeneous failure set"},
			{"slice_index_reference_element.sec", "P15 trivial value surface"},
			{"package16_unsupported/shared_copy.sec", "P16-36"},
			{"package16_unsupported/mutable_move.sec", "P16-36"},
			{"package16_unsupported/holder_reborrow.sec", "P16-36"},
			{"package16_unsupported/returned_subslice.sec", "P16-36"},
			{"package16_unsupported/projected_holder.sec", "P16-36"},
			{"package16_unsupported/text_value_element.sec", "P15 trivial value surface"},
			{"package16_unsupported/nontrivial_replacement.sec", "custom free lifecycle destruction"},
		} {
			t.Run(fmt.Sprintf("%d/%s", width, test.file), func(t *testing.T) {
				program := sliceBoundaryProgram(t, "semantic_ir/package16_source_prelude.sec", "semantic_ir/"+test.file)
				a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				if errs := a.Analyze(program); len(errs) != 0 {
					t.Fatal(errs)
				}
				m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
				var unsupported *UnsupportedFeatureError
				if m != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, test.want) {
					t.Fatalf("dependency lost its capability boundary or leaked a module: %v", err)
				}
			})
		}
	}
}
