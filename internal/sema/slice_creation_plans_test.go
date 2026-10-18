package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Creation queries expose source and endpoint facts with the P15 borrow's own
// identity. The query never recreates a borrow or derives a fresh proof.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 33–38.
func TestResolvedSliceCreationPlans(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_creation_plans_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "slice-creation.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	before := len(a.placeIdentities.sliceRanges) + len(a.placeIdentities.borrows) + len(a.borrows) + len(a.resolvedSliceCreationPlans)
	seen := map[string]ResolvedSliceCreationPlan{}
	for _, statement := range parsed.Program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		for _, statement := range fn.Body.Statements {
			binding, ok := statement.(*ast.LetStatement)
			if !ok {
				continue
			}
			ref, ok := binding.Value.(*ast.RefExpression)
			if !ok {
				continue
			}
			plan, ok := a.ResolvedSliceCreationPlanOf(ref)
			if _, slice := ref.Value.(*ast.SliceExpression); !slice {
				if ok {
					t.Fatal("direct reference acquired a slice creation plan")
				}
				continue
			}
			if !ok {
				t.Fatalf("missing plan at line %d", ref.Token.Line)
			}
			name := binding.Name.Value
			seen[name] = cloneSliceCreationPlan(plan)
			p15, _ := a.ResolvedReferencePlanOf(ref)
			if plan.BorrowID == 0 || plan.BorrowID != p15.Facts.Borrow || plan.LifetimeID != p15.Facts.Lifetime {
				t.Fatalf("%s lost its existing borrow facts", name)
			}
			if plan.ErrorType.Name != "RangeError" || plan.ErrorType.Module != "core" || !plan.ErrorType.ErrorAssignable ||
				!reflect.DeepEqual(plan.ErrorType.EnumValues, []string{"InvalidRange", "StartAfterEnd", "OutOfBounds"}) {
				t.Fatalf("%s error identity = %+v", name, plan.ErrorType)
			}
			again, _ := a.ResolvedSliceCreationPlanOf(ref)
			if !reflect.DeepEqual(plan, again) {
				t.Fatal("query changed facts")
			}
			plan.ErrorType.EnumValues[0] = "Changed"
			value := plan.ErrorType.EnumConsts["OutOfBounds"]
			if value.Value == nil || value.Value.Int64() != 2 {
				t.Fatal("RangeError case values were lost")
			}
			value.Value.SetInt64(999)
			plan.ReferenceFacts.Facts.OriginPlaces[0].Root = "Changed"
			if plan.SourcePlace.Type.Element != nil {
				plan.SourcePlace.Type.Element.Name = "Changed"
			}
			if plan.Start.Constant != nil {
				plan.Start.Constant.SetInt64(999)
			}
			if plan.SourceLength != nil {
				plan.SourceLength.SetInt64(999)
			}
			if plan.StaticRange != nil {
				plan.StaticRange.Start.SetInt64(999)
				plan.StaticRange.EndExclusive.SetInt64(999)
			}
			fresh, _ := a.ResolvedSliceCreationPlanOf(ref)
			if !reflect.DeepEqual(again, fresh) {
				t.Fatalf("query returned mutable aliases for %s", name)
			}
		}
	}
	if len(seen) != 8 {
		t.Fatalf("plans = %v", seen)
	}
	for name, bounds := range map[string][2]int64{"all": {0, 4}, "inclusive": {1, 3}, "view": {1, 4}, "tail": {1, 3}, "empty": {0, 0}} {
		plan := seen[name]
		if plan.CheckKind != SliceRangeProvenSafe || plan.StaticRange == nil ||
			plan.StaticRange.Start.Cmp(big.NewInt(bounds[0])) != 0 || plan.StaticRange.EndExclusive.Cmp(big.NewInt(bounds[1])) != 0 {
			t.Fatalf("%s normalized plan = %+v", name, plan)
		}
	}
	if seen["all"].Start.Present || seen["all"].End.Present || seen["all"].ProofKind != "source-open-bound" ||
		seen["inclusive"].Mode != SliceMutable || seen["inclusive"].EndExclusive || seen["inclusive"].End.Constant.Int64() != 2 ||
		seen["tail"].SourceKind != SliceSourceSlice || !seen["tail"].ReferenceFacts.Reborrow {
		t.Fatal("source spelling/authority/reborrow facts were not retained")
	}
	wide := seen["dynamic"]
	if wide.CheckKind != SliceRangeRuntimeCheck || wide.StaticRange != nil || wide.DynamicRange == 0 ||
		wide.Start.Type.Name != "int128" || !wide.Start.Signed || wide.End.Type.Name != "uint256" || wide.End.Signed {
		t.Fatalf("wide dynamic endpoints = %+v", wide)
	}
	if seen["unknown"].CheckKind != SliceRangeRuntimeCheck || seen["unknown"].SourceKind != SliceSourceSlice ||
		seen["ownedView"].CheckKind != SliceRangeRuntimeCheck || seen["ownedView"].SourceKind != SliceSourceDynamicArray {
		t.Fatal("unknown source extent acquired a safety proof")
	}
	if after := len(a.placeIdentities.sliceRanges) + len(a.placeIdentities.borrows) + len(a.borrows) + len(a.resolvedSliceCreationPlans); after != before {
		t.Fatal("queries changed analyzer state")
	}
	if _, ok := a.ResolvedSliceCreationPlanOf(nil); ok {
		t.Fatal("nil has a plan")
	}
}

// Loader-proven core declarations refine the same RangeError identity that
// standalone Sema supplies to slice creation plans.
// Rules: rules/library/core-library.md — core error types;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 18, 36.
func TestSliceRangeErrorCoreIdentity(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_range_error_core.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "range-error-core.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	parsed.Program.SourceProvenance = map[string]ast.SourceProvenance{"range-error-core.sec": ast.SourceCore}
	a := NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	actual := a.types["RangeError"]
	expected := builtinType("RangeError")
	if !sameConcreteType(actual, expected) || !actual.ErrorAssignable || !reflect.DeepEqual(actual.EnumValues, expected.EnumValues) {
		t.Fatalf("source/fallback identity differs: %+v / %+v", actual, expected)
	}
}

// Endpoint types must be resolved before a usable creation plan is published.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 17, 35–37.
func TestSliceCreationRejectsNonIntegerEndpoints(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_endpoint_type_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "slice bound must be an integer") {
		t.Fatal(errors)
	}
}

// A known parent extent cannot be ignored when deciding a nested range plan.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 18–19, 28, 34–37.
func TestSliceCreationRejectsRangePastKnownParent(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_nested_extent_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "exclusive slice upper bound 4 exceeds length 2") {
		t.Fatal(errors)
	}
}
