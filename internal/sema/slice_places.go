package sema

import (
	"fmt"
	"math/big"
	"sec/internal/ast"
)

// slicePlaceProjection normalizes a source slice to exact half-open bounds: a
// missing start is 0, an inclusive end is end+1 computed exactly, and a
// missing end of a fixed array is its exact length. Non-constant endpoints
// stay unknown.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 13–19, 25–29
func (a *Analyzer) slicePlaceProjection(expr *ast.SliceExpression, containerType Type) PlaceProjection {
	return a.slicePlaceProjectionWithLength(expr, containerType, nil)
}

// slicePlaceProjectionWithLength uses an established source extent for an
// omitted end, including a statically known borrowed subrange. An explicit
// runtime endpoint remains unknown and keeps its dynamic identity. Canonical
// failure classification precedes normalization; invalid constants cannot
// become a normalized static range.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 13–19, 25–30, 59–62.
func (a *Analyzer) slicePlaceProjectionWithLength(expr *ast.SliceExpression, containerType Type, sourceLength *big.Int) PlaceProjection {
	if sourceLength == nil {
		sourceLength, _ = exactFixedArrayLength(containerType)
	}
	start, _ := a.integerConstantValue(expr.Start)
	end, _ := a.integerConstantValue(expr.End)
	normalizedStart, normalizedEnd, _ := normalizeSliceRange(SliceRangeEndpoints{
		Start: start, End: end, StartOmitted: expr.Start == nil, EndOmitted: expr.End == nil,
		Inclusive: !expr.Exclusive, Length: sourceLength,
	})
	projection := PlaceProjection{Kind: PlaceSlice, SliceStart: normalizedStart, SliceEnd: normalizedEnd, SliceEndOpen: expr.End == nil, Token: expr.Token}
	if projection.SliceStart == nil || projection.SliceEnd == nil {
		location := sourceTokenLocation(expr.Token)
		if location.Line > 0 && location.Column > 0 {
			projection.DynamicRange = a.placeIdentities.sliceRange(fmt.Sprintf("slice:%s:%d:%d", location.File, location.Line, location.Column))
		}
	}
	a.recordSequenceProjection(expr, projection)
	return projection
}

// staticSliceExtent returns a detached extent only when every represented
// origin ends in a complete static range with the same length. Joined or
// dynamic origins cannot establish a common source extent by guessing.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 28–30, 59–62.
func staticSliceExtent(place Place) *big.Int {
	if place.AmbiguousProvenance {
		return nil
	}
	var extent *big.Int
	for _, origin := range append([]Place{place}, place.AlternativeOrigins...) {
		if origin.AmbiguousProvenance || len(origin.Projections) == 0 {
			return nil
		}
		last := origin.Projections[len(origin.Projections)-1]
		if last.Kind != PlaceSlice || last.DynamicRange != 0 || last.SliceStart == nil || last.SliceEnd == nil || last.SliceEnd.Cmp(last.SliceStart) < 0 {
			return nil
		}
		length := new(big.Int).Sub(last.SliceEnd, last.SliceStart)
		if extent != nil && extent.Cmp(length) != 0 {
			return nil
		}
		extent = length
	}
	return extent
}

// appendIndexProjection composes a constant index through a static slice
// using arbitrary-precision arithmetic and no int64 index adapter.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §13 "Constant index representation"
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 26–28 (retain symbolic range identity)
func appendIndexProjection(projections []PlaceProjection, index PlaceProjection) []PlaceProjection {
	index = clonePlaceProjection(index)
	if len(projections) == 0 || index.DynamicIndex {
		return append(projections, index)
	}
	last := projections[len(projections)-1]
	if last.Kind != PlaceSlice || last.SliceStart == nil || last.DynamicRange != 0 {
		return append(projections, index)
	}
	constant := placeConstantIndexValue(index.ConstantIndex)
	if constant.Sign() < 0 || (last.SliceEnd != nil && constant.Cmp(new(big.Int).Sub(last.SliceEnd, last.SliceStart)) >= 0) {
		return append(projections, index)
	}
	index.ConstantIndex = new(big.Int).Add(constant, last.SliceStart)
	return append(projections[:len(projections)-1], index)
}

// appendSliceProjection composes a nested static slice into absolute exact
// bounds of its parent; a nested range that is negative or exceeds the
// parent stays a separate projection.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 26–29
func appendSliceProjection(projections []PlaceProjection, child PlaceProjection) []PlaceProjection {
	child = clonePlaceProjection(child)
	if len(projections) == 0 {
		return append(projections, child)
	}
	parent := projections[len(projections)-1]
	if parent.Kind != PlaceSlice || parent.SliceStart == nil || parent.DynamicRange != 0 || child.DynamicRange != 0 {
		return append(projections, child)
	}
	if (child.SliceStart != nil && child.SliceStart.Sign() < 0) || (child.SliceEnd != nil && child.SliceEnd.Sign() < 0) {
		return append(projections, child)
	}
	if parent.SliceEnd != nil {
		length := new(big.Int).Sub(parent.SliceEnd, parent.SliceStart)
		if (child.SliceStart != nil && child.SliceStart.Cmp(length) > 0) || (child.SliceEnd != nil && child.SliceEnd.Cmp(length) > 0) {
			return append(projections, child)
		}
	}
	if child.SliceStart != nil {
		child.SliceStart.Add(child.SliceStart, parent.SliceStart)
	}
	if child.SliceEnd != nil {
		child.SliceEnd.Add(child.SliceEnd, parent.SliceStart)
	} else if parent.SliceEnd != nil {
		child.SliceEnd = new(big.Int).Set(parent.SliceEnd)
	}
	return append(projections[:len(projections)-1], child)
}
