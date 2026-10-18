package sema

import (
	"math/big"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func exactBound(t *testing.T, text string) *big.Int {
	t.Helper()
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		t.Fatalf("invalid bound %q", text)
	}
	return value
}

// Slice projection bounds are exact half-open ranges: endpoints, inclusive
// normalization, nested composition, emptiness, disjointness and index
// containment never depend on host int64 width, and projection copies are
// detached from analyzer-owned storage (P16-09).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 13-19, 25, 27-29
func TestSliceProjectionBoundsAreExactBeyondInt64(t *testing.T) {
	huge := exactBound(t, "9223372036854775807")     // MaxInt64
	beyond := exactBound(t, "9223372036854775808")   // MaxInt64 + 1
	further := exactBound(t, "18446744073709551616") // 2^64

	parent := PlaceProjection{Kind: PlaceSlice, SliceStart: new(big.Int).Set(huge), SliceEnd: new(big.Int).Set(further)}
	child := PlaceProjection{Kind: PlaceSlice, SliceStart: big.NewInt(1), SliceEnd: big.NewInt(3)}
	composed := appendSliceProjection([]PlaceProjection{parent}, child)
	if len(composed) != 1 || composed[0].SliceStart.Cmp(beyond) != 0 || composed[0].SliceEnd.Cmp(exactBound(t, "9223372036854775810")) != 0 {
		t.Fatalf("nested composition is not exact: %+v", composed)
	}
	if child.SliceStart.Cmp(big.NewInt(1)) != 0 || parent.SliceStart.Cmp(huge) != 0 {
		t.Fatal("composition mutated its inputs")
	}

	open := appendSliceProjection([]PlaceProjection{parent}, PlaceProjection{Kind: PlaceSlice, SliceStart: big.NewInt(2)})
	if open[0].SliceEnd.Cmp(further) != 0 || open[0].SliceEnd == parent.SliceEnd {
		t.Fatalf("an open end inherits a detached copy of the parent end: %+v", open[0])
	}

	index := appendIndexProjection([]PlaceProjection{parent}, PlaceProjection{Kind: PlaceIndex, ConstantIndex: big.NewInt(1)})
	if len(index) != 1 || index[0].ConstantIndex.Cmp(beyond) != 0 {
		t.Fatalf("index through a slice is not exact: %+v", index)
	}

	low := PlaceProjection{Kind: PlaceSlice, SliceStart: big.NewInt(0), SliceEnd: new(big.Int).Set(beyond)}
	high := PlaceProjection{Kind: PlaceSlice, SliceStart: new(big.Int).Set(beyond), SliceEnd: new(big.Int).Set(further)}
	overlapping := PlaceProjection{Kind: PlaceSlice, SliceStart: new(big.Int).Set(huge), SliceEnd: new(big.Int).Set(further)}
	if !slicesAreStaticallyDisjoint(low, high) || slicesAreStaticallyDisjoint(low, overlapping) {
		t.Fatal("disjointness beyond int64 is not exact")
	}
	if !slicesAreStaticallyEqual(high, clonePlaceProjection(high)) || slicesAreStaticallyEqual(high, overlapping) {
		t.Fatal("equality beyond int64 is not exact")
	}
	unknownEnd := PlaceProjection{Kind: PlaceSlice, SliceStart: big.NewInt(0)}
	if slicesAreStaticallyEqual(low, unknownEnd) || slicesAreStaticallyDisjoint(unknownEnd, high) {
		t.Fatal("an unknown endpoint must stay conservative")
	}
	if !indexOutsideSlice(PlaceProjection{Kind: PlaceIndex, ConstantIndex: new(big.Int).Set(beyond)}, low) ||
		indexOutsideSlice(PlaceProjection{Kind: PlaceIndex, ConstantIndex: new(big.Int).Set(huge)}, low) {
		t.Fatal("index containment beyond int64 is not exact")
	}

	empty := Place{Projections: []PlaceProjection{{Kind: PlaceSlice, SliceStart: new(big.Int).Set(beyond), SliceEnd: new(big.Int).Set(beyond)}}}
	if !placeIsStaticallyEmpty(empty) || placeIsStaticallyEmpty(Place{Projections: []PlaceProjection{high}}) {
		t.Fatal("emptiness beyond int64 is not exact")
	}

	cloned := clonePlaceProjection(high)
	cloned.SliceStart.SetInt64(0)
	cloned.SliceEnd.SetInt64(0)
	if high.SliceStart.Cmp(beyond) != 0 || high.SliceEnd.Cmp(further) != 0 {
		t.Fatal("cloned slice bounds alias the original")
	}
}

// A source slice normalizes its inclusive end exactly at MaxInt64, where the
// previous int64 model gave up and made the end unknown; a missing start is
// exact zero and a missing end of a fixed array is its exact length (P16-09).
func TestSlicePlaceProjectionNormalizesEndpointsExactly(t *testing.T) {
	p := parser.New(lexer.NewWithFile(`module main

fn Take(values: int[4]) int {
    let inclusive := ref values[9223372036854775807..9223372036854775807]
    let open := ref values[..]
    return 0
}
`, "slice.sec"))
	program := p.Parse()
	if program.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	function := program.Program.Statements[len(program.Program.Statements)-1].(*ast.FunctionDeclaration)
	sliceOf := func(index int) *ast.SliceExpression {
		return function.Body.Statements[index].(*ast.LetStatement).Value.(*ast.RefExpression).Value.(*ast.SliceExpression)
	}
	analyzer := NewAnalyzer()
	inclusive := analyzer.slicePlaceProjection(sliceOf(0), Type{})
	if inclusive.SliceStart == nil || inclusive.SliceStart.String() != "9223372036854775807" ||
		inclusive.SliceEnd == nil || inclusive.SliceEnd.String() != "9223372036854775808" {
		t.Fatalf("inclusive end is not normalized exactly: start=%v end=%v", inclusive.SliceStart, inclusive.SliceEnd)
	}
	// A fixed length beyond int64 (the legacy cache cannot hold it).
	fixed := Type{Name: "int[18446744073709551616]", Kind: ArrayType, Element: &Type{Name: "int", Kind: IntType},
		ArrayShape: ArrayShapeFixed, ArrayLengthDecimal: "18446744073709551616"}
	open := analyzer.slicePlaceProjection(sliceOf(1), fixed)
	if open.SliceStart == nil || open.SliceStart.Sign() != 0 {
		t.Fatalf("a missing start is exact zero: %v", open.SliceStart)
	}
	if open.SliceEnd == nil || open.SliceEnd.String() != "18446744073709551616" {
		t.Fatalf("a missing end of a fixed array is its exact length: %v", open.SliceEnd)
	}
}

// MD-089: the diagnostic presentation of a slice projection distinguishes a
// runtime-unknown start (`*`) from a constant start of zero (omitted), prints
// known bounds exactly and an unknown end as the open `..` form; presentation
// never changes the Place's exact bounds.
//
// Rules:
//   - rules/memory/borrowing.md — § 12(9) slice Place presentation (MD-089)
//   - rules/tooling/diagnostics.md — § 10(7) Place rendering (MD-089)
func TestSlicePlacePresentationDistinguishesUnknownBounds(t *testing.T) {
	bound := func(value int64) *big.Int { return big.NewInt(value) }
	tests := []struct {
		start, end *big.Int
		want       string
	}{
		{bound(0), bound(3), "values[..<3]"},
		{bound(2), bound(3), "values[2..<3]"},
		{nil, bound(3), "values[*..<3]"},
		{bound(2), nil, "values[2..]"},
		{bound(0), nil, "values[..]"},
		{nil, nil, "values[*..]"},
		{bound(0), new(big.Int).Lsh(big.NewInt(1), 70), "values[..<1180591620717411303424]"},
	}
	for _, test := range tests {
		place := Place{Root: "values", Projections: []PlaceProjection{{Kind: PlaceSlice, SliceStart: test.start, SliceEnd: test.end}}}
		if got := place.String(); got != test.want {
			t.Errorf("start=%v end=%v: got %q, want %q", test.start, test.end, got, test.want)
		}
		if (place.Projections[0].SliceStart == nil) != (test.start == nil) {
			t.Errorf("presentation changed the start bound identity of %q", test.want)
		}
	}
}
