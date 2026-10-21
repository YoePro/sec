package semantic

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// The opt-in P16 route leaves slice-free P14/P15 programs identical and does
// not change the default or the established earlier-package rejection paths.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§115–116,128–137.
func TestPackage16BuilderPreservesPredecessorRoutes(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/package16_boundary_plain.sec")
	var previous string
	for _, pkg := range []uint8{15, 16} {
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: pkg})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		if previous != "" && previous != Format(module) {
			t.Fatal("P16 changed the P15 reference/scalar route")
		}
		previous = Format(module)
	}
	program, a = sliceBoundaryInput(t, "semantic_ir/package16_boundary_fixed.sec")
	for _, pkg := range []uint8{0, 14, 15} {
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: pkg})
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) {
			t.Fatalf("package %d: partial module or missing capability error: %v", pkg, err)
		}
		if pkg == 0 && unsupported.Package != 14 || pkg != 0 && unsupported.Package != pkg {
			t.Fatalf("wrong package boundary: %v", err)
		}
		if strings.Contains(unsupported.Feature, "P16-") {
			t.Fatalf("changed predecessor diagnostic: %v", err)
		}
	}
}

// A completed analysis supplies detached creation and index/use facts for the
// entire requested module, including unused bodies. Snapshotting performs no
// re-inference and cannot expose a partially built module when lowering stops.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§33–37,51,128–129.
func TestPackage16SnapshotsResolvedSlicePlans(t *testing.T) {
	program, a := sliceBoundaryInput(t, "sema/slice_plan_queries_valid.sec")
	b := &builder{module: &Module{Identity: "main", Types: NewTypeTable()}, analyzer: a, maxPackage: 16}
	if err := b.prepareSliceLowering(program); err != nil {
		t.Fatal(err)
	}
	if len(b.slicePlans.creations) != 2 || len(b.slicePlans.indexes) != 1 {
		t.Fatal("missing prepared slice creation/index/use facts")
	}
	for expr, plan := range b.slicePlans.creations {
		original, _ := a.ResolvedSliceCreationPlanOf(expr)
		if !plan.HasValidEvaluationOrder() {
			t.Fatal("snapshot lost source operand evaluation order")
		}
		if plan.BorrowID != original.BorrowID || plan.LifetimeID != original.LifetimeID || plan.ReferenceFacts.Facts.Validity != original.ReferenceFacts.Facts.Validity {
			t.Fatal("snapshot changed reference facts")
		}

		plan.EvaluationOrder[0] = sema.SliceOperandEnd
		freshOrder, _ := a.ResolvedSliceCreationPlanOf(expr)
		if !freshOrder.HasValidEvaluationOrder() {
			t.Fatal("prepared operand schedule aliases canonical Sema data")
		}
		if plan.StaticRange != nil {
			plan.StaticRange.Start.SetInt64(999)
			fresh, _ := a.ResolvedSliceCreationPlanOf(expr)
			if fresh.StaticRange.Start.Int64() == 999 {
				t.Fatal("prepared range aliases canonical Sema data")
			}
		}
	}
	for expr, plan := range b.slicePlans.indexes {
		original, _ := a.ResolvedSliceIndexPlanOf(expr)
		if plan.UseKind != original.UseKind || plan.Action != original.Action || plan.IndexType.Name != original.IndexType.Name {
			t.Fatal("snapshot lost resolved element use")
		}
	}
	// A fresh preparation discards mutated local snapshots and consumes only
	// canonical facts; it neither changes the analyzer nor selects a backend.
	if err := b.prepareSliceLowering(program); err != nil {
		t.Fatal(err)
	}
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}

// Missing creation or index plans fail before a previously built function or
// an earlier supported/unsupported expression can hide the semantic gap.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,51,128–129.
func TestPackage16RejectsMissingPlansWithoutPartialIR(t *testing.T) {
	for _, index := range []bool{false, true} {
		program, a := sliceBoundaryInput(t, "sema/slice_plan_queries_valid.sec")
		changed := false
		if err := walk.Inspect(program, func(node any) error {
			ref, ok := node.(*ast.RefExpression)
			if !ok || changed {
				return nil
			}
			if index {
				if old, ok := ref.Value.(*ast.IndexExpression); ok {
					replacement := *old
					ref.Value = &replacement
					changed = true
				}
			} else if _, ok := ref.Value.(*ast.SliceExpression); ok {
				// Mutate the existing source node to retain program identity while
				// replacing the analyzed node used as the public query key.
				for _, statement := range program.Statements {
					if fn, ok := statement.(*ast.FunctionDeclaration); ok {
						for _, stmt := range fn.Body.Statements {
							if binding, ok := stmt.(*ast.LetStatement); ok && binding.Value == ref {
								replacement := *ref
								binding.Value = &replacement
								changed = true
							}
						}
					}
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("missing mutation target")
		}
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		want := "missing resolved slice creation plan"
		if index {
			want = "missing resolved slice index plan"
		}
		var unsupported *UnsupportedFeatureError
		if module != nil || err == nil || !strings.Contains(err.Error(), want) || errors.As(err, &unsupported) {
			t.Fatalf("missing proof conflated with capability boundary: %v", err)
		}
	}
}

// Dynamic owners and slice signatures keep their actual P18/P16-36 gates;
// unrelated modules are not preflighted or lowered for the requested module.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§115–116,123,128–137.
func TestPackage16DependencyAndModuleBoundaries(t *testing.T) {
	for _, test := range []struct{ file, want string }{
		{"semantic_ir/package16_boundary_owner.sec", "dynamic"},
		{"semantic_ir/slice_length_forms.sec", "P16-36"},
	} {
		program, a := sliceBoundaryInput(t, test.file)
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || unsupported.Package != 16 || !strings.Contains(unsupported.Feature, test.want) {
			t.Fatalf("%s: %v", test.file, err)
		}
	}
	program, a := sliceBoundaryInput(t, "semantic_ir/package16_boundary_owner.sec")
	b := &builder{module: &Module{Identity: "main", Types: NewTypeTable()}, analyzer: a, maxPackage: 16}
	if err := b.prepareSliceLowering(program); err != nil {
		t.Fatal(err)
	}
	for ref := range b.slicePlans.creations {
		_, handled, err := (&functionBuilder{owner: b}).p16SliceExpression(ref)
		if !handled || err == nil || !strings.Contains(err.Error(), "P18") {
			t.Fatalf("owner reinterpreted as a slice: %v", err)
		}
	}
	program, a = sliceBoundaryInput(t, "semantic_ir/package16_boundary_modules.sec", "semantic_ir/package16_boundary_other.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
	if err != nil || module == nil || len(module.Functions) != 1 || module.Functions[0].Name != "Ready" {
		t.Fatalf("other module affected selection: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}

// Parse real source fixtures without choosing a scalar plan or running Sema,
// allowing the same source matrix to exercise independent target plans.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34.
func sliceBoundaryProgram(t *testing.T, files ...string) *ast.Program {
	t.Helper()
	program := &ast.Program{}
	for _, file := range files {
		data, err := os.ReadFile("../../../testdata/" + file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, parsed.Statements...)
	}
	return program
}

// Analyze fixture source with the default plan for existing boundary tests.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34.
func sliceBoundaryInput(t *testing.T, files ...string) (*ast.Program, *sema.Analyzer) {
	t.Helper()
	program := sliceBoundaryProgram(t, files...)
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	return program, a
}
