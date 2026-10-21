package semantic

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/sema"
)

// Fresh analyzers must reproduce slice identities, source locations, exact
// ranges and ordered origins. Interleaved plans and reversed metadata-map
// insertion cannot change either the shared AST or previously built modules.
// Rules: rules/compiler/semantic_ir.md — §§6(6),107(1–3),126.26;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§25–30,36–37,141,143.
func TestPackage16FreshAnalysisDeterminism(t *testing.T) {
	for _, file := range []string{
		"slice_source_fixed.sec", "slice_source_reborrow.sec",
		"slice_source_length.sec", "slice_source_fallible.sec",
		"slice_source_index.sec", "slice_source_wide.sec",
	} {
		t.Run(file, func(t *testing.T) {
			program := sliceBoundaryProgram(t, "semantic_ir/"+file)
			originalAST := sliceASTBytes(t, program)
			baseline := map[uint16]string{}
			plans := map[uint16]*sliceLoweringPlans{}
			var previous []*Module
			var previousText []string
			// Revisit each plan after the other has consumed the same syntax.
			for run, width := range []uint16{64, 32, 64, 32} {
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
				if m.PointerWidthBits != width {
					t.Fatal("module lost its CompilationPlan width")
				}
				text := Format(m)
				if run >= 2 {
					// Distinct parser objects also rule out identities derived
					// from syntax-node addresses rather than source semantics.
					fresh := sliceBoundaryProgram(t, "semantic_ir/"+file)
					freshAnalyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
					if errs := freshAnalyzer.Analyze(fresh); len(errs) != 0 {
						t.Fatal(errs)
					}
					freshModule, err := Build(fresh, freshAnalyzer, BuildOptions{RequestedModule: "main", MaxPackage: 16})
					if err != nil {
						t.Fatal(err)
					}
					if err := Verify(freshModule); err != nil {
						t.Fatal(err)
					}
					if Format(freshModule) != text {
						t.Fatal("fresh parsing changed semantic identities or IR")
					}
				}
				b := sliceDeterminismBuilder(t, program, a, width)
				if old, ok := baseline[width]; ok {
					if text != old || !reflect.DeepEqual(plans[width], b.slicePlans) {
						t.Fatalf("%d-bit fresh analysis changed IR or resolved range/borrow/origin facts", width)
					}
				} else {
					baseline[width], plans[width] = text, b.slicePlans
				}
				// Query in reverse syntax order and reinsert the detached map
				// records in that order, without reordering semantic origin lists.
				var expressions []ast.Expression
				if err := walk.Inspect(program, func(node any) error {
					if expr, ok := node.(ast.Expression); ok {
						expressions = append(expressions, expr)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				slices.Reverse(expressions)
				reordered := &sliceLoweringPlans{
					creations: map[*ast.RefExpression]sema.ResolvedSliceCreationPlan{},
					indexes:   map[*ast.IndexExpression]sema.ResolvedSliceIndexPlan{},
					lengths:   map[ast.Expression]sema.ResolvedSliceLengthPlan{},
				}
				for _, expr := range expressions {
					if ref, ok := expr.(*ast.RefExpression); ok {
						if plan, found := a.ResolvedSliceCreationPlanOf(ref); found {
							reordered.creations[ref] = plan
						}
					}
					if index, ok := expr.(*ast.IndexExpression); ok {
						if plan, found := a.ResolvedSliceIndexPlanOf(index); found {
							reordered.indexes[index] = plan
						}
					}
					if plan, found := a.ResolvedSliceLengthPlanOf(expr); found {
						reordered.lengths[expr] = plan
					}
				}
				if !reflect.DeepEqual(b.slicePlans, reordered) || len(reordered.creations) == 0 {
					t.Fatal("reordered read-only queries changed slice metadata")
				}
				b.slicePlans = reordered
				for _, statement := range program.Statements {
					if fn, ok := statement.(*ast.FunctionDeclaration); ok {
						if err := b.buildFunction(fn); err != nil {
							t.Fatal(err)
						}
					}
				}
				if err := Verify(b.module); err != nil {
					t.Fatal(err)
				}
				if Format(b.module) != text || !reflect.DeepEqual(plans[width], b.slicePlans) {
					t.Fatalf("%d-bit reversed metadata changed IR or mutated resolved plans", width)
				}
				if !bytes.Equal(originalAST, sliceASTBytes(t, program)) {
					t.Fatalf("run %d mutated shared source syntax", run)
				}
				previous, previousText = append(previous, m), append(previousText, text)
				for i, old := range previous {
					if Format(old) != previousText[i] {
						t.Fatal("fresh analysis/build mutated a previously built module")
					}
				}
			}
			// Abstract printed operations may match across plans; the actual
			// modules must keep independent, unmodified target facts.
			for i, m := range previous {
				if m.PointerWidthBits != []uint16{64, 32, 64, 32}[i] {
					t.Fatal("another plan mutated an earlier module's target facts")
				}
			}
		})
	}
}

// Construct the ordinary detached plan preflight independently of Build so
// tests can vary only map insertion order before the real function producer.
// Rules: rules/compiler/semantic_ir.md — §§64(2–5),107;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§33–37,128–129.
func sliceDeterminismBuilder(t *testing.T, program *ast.Program, a *sema.Analyzer, width uint16) *builder {
	t.Helper()
	b := &builder{
		module:   &Module{Version: Version, Identity: "main", Types: NewTypeTable(), PointerWidthBits: width},
		analyzer: a, maxPackage: 16,
		definedEnums: map[TypeID]bool{}, definedUnions: map[TypeID]bool{}, definedStructs: map[TypeID]bool{},
	}
	if err := b.prepareSliceLowering(program); err != nil {
		t.Fatal(err)
	}
	return b
}

// Capture all syntax/provenance, rather than only tokens or node addresses, to
// catch mutation when the same parsed program is analyzed for another target.
// Rules: rules/compiler/semantic_ir.md — §§6(6),107(1–3).
func sliceASTBytes(t *testing.T, program *ast.Program) []byte {
	t.Helper()
	data, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Invalid slice source must reproduce diagnostic codes, ranges, causes, help
// and ordering across fresh/interleaved plans, without relying on Error()'s
// abbreviated human string. Invalid fixtures never count as successful IR.
// Rules: rules/compiler/semantic_ir.md — §107(2–3);
// rules/tooling/diagnostics.md — §§8–10;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§18–19,38,51–52.
func TestPackage16DiagnosticDeterminism(t *testing.T) {
	for _, file := range []string{
		"slice_determinism_invalid.sec", "slice_nested_extent_invalid.sec",
		"slice_endpoint_type_invalid.sec", "slice_empty_local_return_invalid.sec",
	} {
		t.Run(file, func(t *testing.T) {
			program := sliceBoundaryProgram(t, "sema/"+file)
			originalAST := sliceASTBytes(t, program)
			baseline := map[uint16][]sema.Error{}
			for _, width := range []uint16{32, 64, 32, 64} {
				a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				errs := a.Analyze(program)
				if len(errs) == 0 {
					t.Fatal("invalid source unexpectedly passed analysis")
				}
				if file == "slice_determinism_invalid.sec" {
					for _, want := range []string{
						"slice index 2 is out of bounds", "slice index 0 is out of bounds",
						"slice index -1 is out of bounds",
						"exclusive slice upper bound 340282366920938463463374607431768211456 exceeds length 4",
					} {
						if !slices.ContainsFunc(errs, func(e sema.Error) bool { return strings.Contains(e.Message, want) }) {
							t.Fatalf("missing exact source diagnostic %q: %v", want, errs)
						}
					}
				}
				if old, ok := baseline[width]; ok && !reflect.DeepEqual(old, errs) {
					t.Fatalf("%d-bit diagnostics changed: %v -> %v", width, old, errs)
				}
				baseline[width] = errs
				if !bytes.Equal(originalAST, sliceASTBytes(t, program)) {
					t.Fatal("diagnostic analysis mutated shared source syntax")
				}
			}
			for width, errs := range baseline {
				for _, diagnostic := range errs {
					if diagnostic.Line <= 0 || diagnostic.Column <= 0 {
						t.Fatalf("%d-bit diagnostic lost its source location: %+v", width, diagnostic)
					}
				}
			}
		})
	}
}

// Multiple possible backing roots must keep the same ordered external origins
// and exact composed ranges after fresh analyses on interleaved target plans.
// This is Sema evidence only: slice-valued signatures remain P16-36.
// Rules: rules/compiler/semantic_ir.md — §§107,126.22–23,26;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§25–30,66–69.
func TestPackage16OriginOrderDeterminism(t *testing.T) {
	program := sliceBoundaryProgram(t, "sema/slice_determinism_origins_valid.sec")
	originalAST := sliceASTBytes(t, program)
	var baseline *sliceLoweringPlans
	for _, width := range []uint16{64, 32, 64, 32} {
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		b := sliceDeterminismBuilder(t, program, a, width)
		if baseline == nil {
			baseline = b.slicePlans
		} else if !reflect.DeepEqual(baseline, b.slicePlans) {
			t.Fatal("external backing-root order or exact ranges changed across analyses")
		}
		found := false
		for _, plan := range b.slicePlans.creations {
			if len(plan.SourcePlace.AlternativeOrigins) == 0 {
				continue
			}
			found = true
			origins := plan.ReferenceFacts.Facts.OriginPlaces
			if len(origins) != 2 || origins[0].String() != "left[2..<4]" || origins[1].String() != "right[3..<5]" {
				t.Fatalf("multiple backing roots lost their exact composed ranges/order: %v", origins)
			}
		}
		if !found {
			t.Fatal("fixture no longer exercises multiple backing roots")
		}
		if !bytes.Equal(originalAST, sliceASTBytes(t, program)) {
			t.Fatal("origin analysis mutated shared source syntax")
		}
	}
}
