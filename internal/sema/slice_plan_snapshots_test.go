package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Both public slice plan queries are detached from the analyzer and from one
// another, including nested type graphs, finite origin trees, exact values
// and P15 reference facts. Querying requires no syntax or inference caches.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§25,37,51.
func TestSlicePlanQueriesAreDetachedAndReadOnly(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/slice_plan_queries_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "slice-queries.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(a.resolvedSliceCreationPlans) != 2 || len(a.resolvedSliceIndexPlans) != 1 {
		t.Fatal("missing fixture plans")
	}

	// Supply a finite, nested alternative tree to all three origin-bearing
	// surfaces. A query must preserve each alternative before detaching it.
	// This catches destructive flattening by a more general Place clone.
	leaf := Place{Root: "deep-origin", Type: Type{Kind: StructType, Fields: []StructField{{Name: "Samples", Type: NewFixedArrayType(Type{Name: "int", Kind: IntType}, big.NewInt(4))}}},
		Projections: []PlaceProjection{{Kind: PlaceSlice, SliceStart: big.NewInt(1), SliceEnd: big.NewInt(4), IndexType: Type{Kind: ReferenceType, Element: &Type{Name: "int", Kind: IntType}}}}}
	tree := Place{Root: "alternative", AlternativeOrigins: []Place{leaf}}
	for expr, plan := range a.resolvedSliceCreationPlans {
		plan.SourcePlace.AlternativeOrigins = []Place{tree}
		plan.ReferenceFacts.Place.AlternativeOrigins = []Place{tree}
		plan.ReferenceFacts.Facts.OriginPlaces[0].AlternativeOrigins = []Place{tree}
		a.resolvedSliceCreationPlans[expr] = plan
	}
	for expr, plan := range a.resolvedSliceIndexPlans {
		plan.SourcePlace.AlternativeOrigins = []Place{tree}
		a.resolvedSliceIndexPlans[expr] = plan
	}

	identities := a.placeIdentities.clone()
	counts := []int{len(a.borrows), len(a.resolvedReferencePlans), len(a.resolvedSliceCreationPlans), len(a.resolvedSliceIndexPlans), len(a.errors)}
	// Empty caches make accidental re-inference or place resolution visible.
	types, places := a.expressionTypes, a.resolvedPlaces
	a.expressionTypes = map[ast.Expression]Type{}
	a.resolvedPlaces = map[ast.Expression]Place{}
	defer func() { a.expressionTypes, a.resolvedPlaces = types, places }()

	for expr := range a.resolvedSliceCreationPlans {
		original, _ := a.ResolvedSliceCreationPlanOf(expr)
		if len(original.SourcePlace.AlternativeOrigins[0].AlternativeOrigins) != 1 ||
			len(original.ReferenceFacts.Place.AlternativeOrigins[0].AlternativeOrigins) != 1 ||
			len(original.ReferenceFacts.Facts.OriginPlaces[0].AlternativeOrigins[0].AlternativeOrigins) != 1 {
			t.Fatal("query discarded a nested alternative origin")
		}
		for repeat := 0; repeat < 3; repeat++ {
			changed, _ := a.ResolvedSliceCreationPlanOf(expr)
			changed.EvaluationOrder[0] = SliceOperandEnd
			mutateSliceQueryType(&changed.ElementType)
			mutateSliceQueryType(&changed.SourceSliceType)
			mutateSliceQueryType(&changed.Start.Type)
			mutateSliceQueryType(&changed.End.Type)
			mutateSliceQueryType(&changed.ReferenceFacts.Facts.Referent)
			mutateSliceQueryPlace(&changed.SourcePlace)
			mutateSliceQueryPlace(&changed.ReferenceFacts.Place)
			for i := range changed.ReferenceFacts.Facts.OriginPlaces {
				mutateSliceQueryPlace(&changed.ReferenceFacts.Facts.OriginPlaces[i])
			}
			if len(changed.ReferenceFacts.Facts.StorageIdentities) > 0 {
				changed.ReferenceFacts.Facts.StorageIdentities[0] = 999
			}
			for _, integer := range []*big.Int{changed.SourceLength, changed.Start.Constant, changed.End.Constant} {
				if integer != nil {
					integer.SetInt64(999)
				}
			}
			if changed.StaticRange != nil {
				changed.StaticRange.Start.SetInt64(999)
				changed.StaticRange.EndExclusive.SetInt64(999)
			}
			changed.ErrorType.EnumValues[0] = "Changed"
			changed.ErrorType.EnumConsts["OutOfBounds"].Value.SetInt64(999)
			fresh, _ := a.ResolvedSliceCreationPlanOf(expr)
			if !reflect.DeepEqual(original, fresh) {
				t.Fatal("creation query exposed mutable data")
			}
		}
	}
	for expr := range a.resolvedSliceIndexPlans {
		original, _ := a.ResolvedSliceIndexPlanOf(expr)
		if len(original.SourcePlace.AlternativeOrigins[0].AlternativeOrigins) != 1 {
			t.Fatal("index query discarded nested origin")
		}
		for repeat := 0; repeat < 3; repeat++ {
			changed, _ := a.ResolvedSliceIndexPlanOf(expr)
			mutateSliceQueryType(&changed.SliceType)
			mutateSliceQueryType(&changed.ElementType)
			mutateSliceQueryType(&changed.IndexType)
			mutateSliceQueryPlace(&changed.SourcePlace)
			if changed.ConstantIndex != nil {
				changed.ConstantIndex.SetInt64(999)
			}
			if changed.SliceLength != nil {
				changed.SliceLength.SetInt64(999)
			}
			changed.ErrorType.EnumValues[0] = "Changed"
			changed.ErrorType.EnumConsts["OutOfBounds"].Value.SetInt64(999)
			fresh, _ := a.ResolvedSliceIndexPlanOf(expr)
			if !reflect.DeepEqual(original, fresh) {
				t.Fatal("index query exposed mutable data")
			}
		}
	}
	if _, ok := a.ResolvedSliceCreationPlanOf(&ast.RefExpression{}); ok {
		t.Fatal("unknown creation inferred a plan")
	}
	if _, ok := a.ResolvedSliceIndexPlanOf(&ast.IndexExpression{}); ok {
		t.Fatal("unknown index inferred a plan")
	}
	var absent *Analyzer
	if _, ok := absent.ResolvedSliceCreationPlanOf(nil); ok {
		t.Fatal("nil analyzer has a creation plan")
	}
	if _, ok := absent.ResolvedSliceIndexPlanOf(nil); ok {
		t.Fatal("nil analyzer has an index plan")
	}
	after := []int{len(a.borrows), len(a.resolvedReferencePlans), len(a.resolvedSliceCreationPlans), len(a.resolvedSliceIndexPlans), len(a.errors)}
	if !reflect.DeepEqual(identities, a.placeIdentities) || !reflect.DeepEqual(counts, after) || len(a.expressionTypes) != 0 || len(a.resolvedPlaces) != 0 {
		t.Fatal("plan query changed analyzer state")
	}
}

// Mutate nested containers as well as leaves so shallow copies fail the
// snapshot test (rules/mlir/packages/sec-mlir-dialect_package16.md — §37).
func mutateSliceQueryType(typ *Type) {
	typ.Name = "Changed"
	if typ.Element != nil {
		mutateSliceQueryType(typ.Element)
	}
	for i := range typ.TypeArgs {
		mutateSliceQueryType(&typ.TypeArgs[i])
	}
	for i := range typ.Fields {
		typ.Fields[i].Name = "Changed"
		mutateSliceQueryType(&typ.Fields[i].Type)
	}
	for i := range typ.ReferenceOriginAlternatives {
		typ.ReferenceOriginAlternatives[i].Name = "Changed"
	}
}

// Mutate every reachable exact projection and alternative, preserving no
// aliases to the snapshot (rules/mlir/packages/sec-mlir-dialect_package16.md — §§25,37).
func mutateSliceQueryPlace(place *Place) {
	place.Root = "Changed"
	mutateSliceQueryType(&place.Type)
	for i := range place.Projections {
		projection := &place.Projections[i]
		mutateSliceQueryType(&projection.IndexType)
		for _, integer := range []*big.Int{projection.SliceStart, projection.SliceEnd, projection.ArrayLength, projection.ConstantIndex} {
			if integer != nil {
				integer.SetInt64(999)
			}
		}
	}
	for i := range place.AlternativeOrigins {
		mutateSliceQueryPlace(&place.AlternativeOrigins[i])
	}
}
