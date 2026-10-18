package sema

import (
	"math/big"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Static nested ranges use their parent's exact extent for omitted ends,
// remain disjoint where proven, and empty borrows keep origin/lifetime facts
// even though their backing elements remain available.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 28–30, 59–62.
func TestStaticSliceSubrangesAndEmptyProvenance(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_static_subranges_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "static-slices.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if len(analyzer.placeIdentities.sliceRanges) != 0 {
		t.Fatal("fully static subranges allocated dynamic range identities")
	}
	for _, test := range []struct {
		function, statement int
		start, end          int64
		empty               bool
	}{
		{1, 2, 2, 4, false},
		{1, 3, 4, 7, false},
		{2, 2, 3, 3, true},
		{2, 3, 3, 3, true},
		{3, 1, 0, 0, true},
		{3, 2, 0, 0, true},
	} {
		function := parsed.Program.Statements[test.function].(*ast.FunctionDeclaration)
		ref := function.Body.Statements[test.statement].(*ast.LetStatement).Value.(*ast.RefExpression)
		place, ok := analyzer.ResolvedPlaceOf(ref.Value)
		if !ok || len(place.Projections) != 1 {
			t.Fatalf("missing normalized source Place: %+v", place)
		}
		projection := place.Projections[0]
		if projection.DynamicRange != 0 || projection.SliceStart == nil || projection.SliceEnd == nil ||
			projection.SliceStart.Cmp(big.NewInt(test.start)) != 0 || projection.SliceEnd.Cmp(big.NewInt(test.end)) != 0 {
			t.Fatalf("wrong absolute range: %+v", projection)
		}
		if placeIsStaticallyEmpty(place) != test.empty {
			t.Fatalf("wrong emptiness: %+v", place)
		}
		if test.empty {
			plan, ok := analyzer.ResolvedReferencePlanOf(ref)
			if !ok || plan.Facts.Borrow == 0 || plan.Facts.Lifetime == 0 ||
				len(plan.Facts.OriginPlaces) == 0 || len(plan.Facts.StorageIdentities) == 0 || place.RootID == 0 {
				t.Fatalf("empty borrow lost origin or lifetime: %+v", plan)
			}
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/slice_empty_local_return_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(invalid))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot return reference to local variable values") {
		t.Fatalf("an empty local-origin slice must not escape: %v", errors)
	}
}

// Static extents are exact, detached and unanimous across joined origins;
// unknown/dynamic or unequal origins retain uncertainty.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 25–30, 59–62.
func TestStaticSliceExtentRequiresUnanimousOrigins(t *testing.T) {
	start := exactBound(t, "18446744073709551616")
	end := new(big.Int).Add(start, big.NewInt(5))
	place := Place{Projections: []PlaceProjection{{Kind: PlaceSlice, SliceStart: start, SliceEnd: end}}}
	extent := staticSliceExtent(place)
	if extent == nil || extent.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("wrong exact extent: %v", extent)
	}
	extent.SetInt64(999)
	if new(big.Int).Sub(end, start).Cmp(big.NewInt(5)) != 0 {
		t.Fatal("extent aliases an original bound")
	}
	place.AlternativeOrigins = []Place{clonePlace(place)}
	if staticSliceExtent(place) == nil {
		t.Fatal("equal extents across joined origins should remain exact")
	}
	place.AlternativeOrigins[0].Projections[0].SliceEnd.Add(end, big.NewInt(1))
	if staticSliceExtent(place) != nil {
		t.Fatal("different origin extents must stay unknown")
	}
	place.AlternativeOrigins = nil
	place.Projections[0].DynamicRange = 1
	if staticSliceExtent(place) != nil {
		t.Fatal("a dynamic source cannot provide a static extent")
	}
}
