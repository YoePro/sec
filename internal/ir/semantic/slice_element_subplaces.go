package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// Sub-Places of slice elements (P16-48; package16 §§ 116, 140). A path such
// as `pairs[i].Right` reaches its stored field through the element Place the
// slice index plan produces: the slice source, index validity and bounds
// guard are lowered once by the existing slice element route, and the
// remaining field / fixed-array steps project below that Place with the
// slice's authority. Borrowing it reuses the P15 borrow of a sub-Place.

// sliceElementSubplace forms the Place of a field/element path below a slice
// element. It reports false when expr is not rooted at a slice index.
func (fb *functionBuilder) sliceElementSubplace(expr ast.Expression, projections []sema.PlaceProjection, facts *PlaceMetadata) (builtValue, bool, error) {
	var steps []ast.Expression
	current := expr
	for {
		switch step := current.(type) {
		case *ast.MemberExpression:
			steps = append([]ast.Expression{step}, steps...)
			current = step.Object
			continue
		case *ast.IndexExpression:
			if plan, ok := fb.owner.slicePlans.indexes[step]; ok {
				if len(steps) == 0 {
					return builtValue{}, false, nil
				}
				element, err := fb.lowerSliceElementPlace(step, plan)
				if err != nil {
					return builtValue{}, true, err
				}
				place, err := fb.projectPlace(steps, projections, element, plan.Mutable, facts)
				return place, true, err
			}
			steps = append([]ast.Expression{step}, steps...)
			current = step.Left
			continue
		}
		return builtValue{}, false, nil
	}
}

// lowerSliceElementSubplaceBorrow lowers `ref s[i].f` / `ref mut s[i].f`.
func (fb *functionBuilder) lowerSliceElementSubplaceBorrow(expr *ast.RefExpression) (builtValue, bool, error) {
	reference, ok := fb.owner.analyzer.ResolvedReferencePlanOf(expr)
	if !ok {
		return builtValue{}, false, nil
	}
	place, handled, err := fb.sliceElementSubplace(expr.Value, reference.Place.Projections, placeMetadataOf(reference.Place))
	if !handled || err != nil {
		return builtValue{}, handled, err
	}
	if !reference.Facts.Checkable() || reference.Facts.MutableAuthority != expr.Mutable {
		return builtValue{}, true, fb.unsupported("slice element sub-Place borrow without complete resolved reference facts", expr.Token)
	}
	value, err := fb.emitBorrow(expr, reference, place)
	return value, true, err
}
