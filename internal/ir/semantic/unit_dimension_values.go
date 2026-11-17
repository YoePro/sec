package semantic

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// UnitDimension values that escape compile-time evaluation (MD-062 §§ 7(12),
// 11(5), 12(8)): a function parameter, a local, an aggregate field or a call
// result. A value is the canonical finite mapping from semantic axis identity
// to a non-zero exponent, carried as a static constant in its sorted
// normalized form; equality compares those mappings structurally, so it is
// collision-free and needs no hash, interning, registry or allocation.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types", "Semantic IR and lowering"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(2)–(13), 12(7)–(8)

// TypeUnitDimension is the opaque compiler-known UnitDimension value type.
const TypeUnitDimension TypeKind = "unit-dimension"

const (
	// OpUnitDimensionConstant materializes one canonical dimension.
	OpUnitDimensionConstant OpKind = "unit-dimension.constant"
	// OpUnitDimensionCompare is `==` or `!=` (Operator) of two dimensions.
	OpUnitDimensionCompare OpKind = "unit-dimension.compare"
	// OpUnitDimensionAxisCount reads AxisCount: uint.
	OpUnitDimensionAxisCount OpKind = "unit-dimension.axis-count"
	// OpUnitDimensionIsDimensionless reads IsDimensionless: bool.
	OpUnitDimensionIsDimensionless OpKind = "unit-dimension.is-dimensionless"
)

// UnitDimensionAxis is one normalized axis of a dimension: its canonical
// semantic axis identity (module-qualified for a user/domain axis) and its
// non-zero exponent.
type UnitDimensionAxis struct {
	Axis     string
	Exponent int64
}

// UnitDimensionFacts is the normalized axis list of a dimension constant, in
// ascending axis-identity order.
type UnitDimensionFacts struct {
	Axes []UnitDimensionAxis
}

func unitDimensionFactsOf(dimension sema.Dimension) *UnitDimensionFacts {
	facts := &UnitDimensionFacts{Axes: []UnitDimensionAxis{}}
	for axis, exponent := range dimension.Base {
		if exponent != 0 {
			facts.Axes = append(facts.Axes, UnitDimensionAxis{Axis: axis, Exponent: int64(exponent)})
		}
	}
	sort.Slice(facts.Axes, func(i, j int) bool { return facts.Axes[i].Axis < facts.Axes[j].Axis })
	return facts
}

func formatUnitDimension(facts *UnitDimensionFacts) string {
	if facts == nil {
		return ""
	}
	parts := make([]string, len(facts.Axes))
	for index, axis := range facts.Axes {
		parts[index] = fmt.Sprintf("%s^%d", axis.Axis, axis.Exponent)
	}
	return " dimension=[" + strings.Join(parts, ", ") + "]"
}

// buildUnitDimensionValue materializes a `<U>.Dimension` read that is used as
// a value rather than observed by a folded property or comparison.
func (fb *functionBuilder) buildUnitDimensionValue(expr *ast.MemberExpression, typeID TypeID) (builtValue, error) {
	dimension, constant := fb.owner.analyzer.ResolvedUnitDimensionOf(expr)
	if !constant {
		return builtValue{}, fb.unsupported("UnitDimension value without a resolved dimension", expr.Property.Token)
	}
	if typeID == 0 {
		var err error
		if typeID, err = fb.owner.internType(sema.Type{Name: "UnitDimension", Module: "core", Kind: sema.UnitDimensionType, Named: true, Declared: true}); err != nil {
			return builtValue{}, err
		}
	}
	facts := fb.unitDimensionFacts(expr, "Dimension")
	return fb.result(Operation{Kind: OpUnitDimensionConstant, UnitDimension: unitDimensionFactsOf(dimension), UnitMetadata: facts, Location: locationFromExpression(expr)}, typeID), nil
}

// buildRuntimeUnitDimensionProperty reads AxisCount or IsDimensionless of a
// runtime UnitDimension value.
func (fb *functionBuilder) buildRuntimeUnitDimensionProperty(expr *ast.MemberExpression, axisCount bool, typeID TypeID) (builtValue, error) {
	value, err := fb.buildExpr(expr.Object, 0)
	if err != nil {
		return builtValue{}, err
	}
	kind := OpUnitDimensionIsDimensionless
	if axisCount {
		kind = OpUnitDimensionAxisCount
	}
	return fb.result(Operation{Kind: kind, Operands: []ValueID{value.id}, Location: locationFromExpression(expr)}, typeID), nil
}

// buildRuntimeUnitDimensionEquality compares two UnitDimension values.
func (fb *functionBuilder) buildRuntimeUnitDimensionEquality(infix *ast.InfixExpression, typeID TypeID) (builtValue, error) {
	left, err := fb.buildExpr(infix.Left, 0)
	if err != nil {
		return builtValue{}, err
	}
	right, err := fb.buildExpr(infix.Right, 0)
	if err != nil {
		return builtValue{}, err
	}
	if typeID == 0 {
		if typeID, err = fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType}); err != nil {
			return builtValue{}, err
		}
	}
	return fb.result(Operation{Kind: OpUnitDimensionCompare, Operands: []ValueID{left.id, right.id}, Operator: infix.Operator, Location: locationFromExpression(infix)}, typeID), nil
}

// verifyUnitDimensionOperation checks operand and result types, and that a
// constant is in canonical form: non-zero exponents and strictly ascending,
// therefore unique, axis identities.
func verifyUnitDimensionOperation(module *Module, op Operation, values map[ValueID]Value) error {
	isDimension := func(id TypeID) bool {
		typ, ok := module.Types.Lookup(id)
		return ok && typ.Kind == TypeUnitDimension
	}
	resultIs := func(kinds ...TypeKind) bool {
		return len(op.Results) == 1 && typeHasKind(module.Types, op.Results[0].Type, kinds...)
	}
	switch op.Kind {
	case OpUnitDimensionConstant:
		if len(op.Operands) != 0 || len(op.Results) != 1 || !isDimension(op.Results[0].Type) || op.UnitDimension == nil {
			return fmt.Errorf("unit-dimension.constant needs a dimension and one UnitDimension result")
		}
		for index, axis := range op.UnitDimension.Axes {
			if axis.Axis == "" || axis.Exponent == 0 {
				return fmt.Errorf("unit-dimension.constant axis %d is empty or has a zero exponent", index)
			}
			if index > 0 && op.UnitDimension.Axes[index-1].Axis >= axis.Axis {
				return fmt.Errorf("unit-dimension.constant axes are not canonical (sorted and unique)")
			}
		}
	case OpUnitDimensionCompare:
		if len(op.Operands) != 2 || !isDimension(values[op.Operands[0]].Type) || !isDimension(values[op.Operands[1]].Type) || !resultIs(TypeBool) || op.Operator != "==" && op.Operator != "!=" {
			return fmt.Errorf("unit-dimension.compare needs two UnitDimension operands, == or != and a bool result")
		}
	case OpUnitDimensionAxisCount, OpUnitDimensionIsDimensionless:
		want := TypeUint
		if op.Kind == OpUnitDimensionIsDimensionless {
			want = TypeBool
		}
		if len(op.Operands) != 1 || !isDimension(values[op.Operands[0]].Type) || !resultIs(want) {
			return fmt.Errorf("%s needs one UnitDimension operand and a %s result", op.Kind, want)
		}
	}
	if op.UnitDimension != nil && op.Kind != OpUnitDimensionConstant {
		return fmt.Errorf("%s carries unit-dimension facts", op.Kind)
	}
	return nil
}
