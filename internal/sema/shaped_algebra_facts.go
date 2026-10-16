package sema

import (
	"maps"
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ResolvedShapedMultiply is the canonical static x decision for later IR and
// tooling. Type surfaces retain nominal identity, numeric/unit algebra and exact
// shapes; declaration bodies/defaults are not a second type registry. A proven
// shape says nothing about the scalar operations' overflow or panic behavior.
// Rules: rules/collections/shaped-types.md — §§4–5,22.1–22.4,29,35;
// rules/corrections/applied/correction27-20260823.md — "Bug 2".
type ResolvedShapedMultiply struct {
	Kind                               string // matrix-matrix or matrix-vector
	LeftType, RightType, ResultType    Type
	ProductElement, AccumulatorElement Type
	Rows, Inner, Columns               int64
	ShapeCheck                         string // proven-safe; runtime shapes are not implemented here
	Source                             lexer.Token
}

// finishShapedMultiply validates the derived result extent instead of inheriting
// the left operand's cached count, then publishes the successful static plan.
// Rules: rules/collections/shaped-types.md — §§4–5,22,29;
// rules/corrections/applied/correction27-20260823.md — "Preserve the proven product".
func (a *Analyzer) finishShapedMultiply(expr *ast.InfixExpression, left, right, product, accumulator, result Type) (Type, expressionValue) {
	if !a.validateShapedStaticType(expr.Token, &result, len(result.ConstArgs), len(result.ConstArgs)) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if !a.summaryPass {
		plan := ResolvedShapedMultiply{Kind: "matrix-matrix", LeftType: left, RightType: right, ResultType: result, ProductElement: product, AccumulatorElement: accumulator, Rows: left.ConstArgs[0], Inner: left.ConstArgs[1], Columns: 1, ShapeCheck: "proven-safe", Source: expr.Token}
		if right.Name == "vector" {
			plan.Kind = "matrix-vector"
		} else {
			plan.Columns = right.ConstArgs[1]
		}
		a.resolvedShapedMultiplies[expr] = cloneShapedMultiply(plan)
	}
	return result, expressionValue{Display: expr.String()}
}

// ResolvedShapedMultiplyOf returns detached operator facts without inference,
// diagnostics, new identities or recomputed shape/scalar decisions.
// Rules: rules/compiler/semantic_ir.md — §91 "Semantic plans versus AST";
// rules/collections/shaped-types.md — §§22,29,35.
func (a *Analyzer) ResolvedShapedMultiplyOf(expr *ast.InfixExpression) (ResolvedShapedMultiply, bool) {
	if a == nil || expr == nil {
		return ResolvedShapedMultiply{}, false
	}
	plan, found := a.resolvedShapedMultiplies[expr]
	if !found {
		return ResolvedShapedMultiply{}, false
	}
	return cloneShapedMultiply(plan), true
}

// cloneShapedMultiply detaches all operator-relevant numeric/unit/shape data.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results;
// rules/collections/shaped-types.md — §§22,29,35.
func cloneShapedMultiply(plan ResolvedShapedMultiply) ResolvedShapedMultiply {
	plan.LeftType = shapedAlgebraTypeSnapshot(plan.LeftType)
	plan.RightType = shapedAlgebraTypeSnapshot(plan.RightType)
	plan.ResultType = shapedAlgebraTypeSnapshot(plan.ResultType)
	plan.ProductElement = shapedAlgebraTypeSnapshot(plan.ProductElement)
	plan.AccumulatorElement = shapedAlgebraTypeSnapshot(plan.AccumulatorElement)
	return plan
}

// shapedAlgebraTypeSnapshot extends the common semantic type surface with the
// exact shape, carrier limits and unit algebra needed by a resolved x consumer.
// Rules: rules/collections/shaped-types.md — §§4–5,22;
// rules/types/units.md — "Arithmetic"; rules/compiler/compiler_analysis.md — immutable results.
func shapedAlgebraTypeSnapshot(typ Type) Type {
	result := semanticSnapshotType(typ)
	result.ConstArgs = append([]int64(nil), typ.ConstArgs...)
	if typ.StaticElementCount != nil {
		result.StaticElementCount = new(big.Int).Set(typ.StaticElementCount)
	}
	if typ.MinInteger != nil {
		result.MinInteger = new(big.Int).Set(typ.MinInteger)
	}
	if typ.MaxInteger != nil {
		result.MaxInteger = new(big.Int).Set(typ.MaxInteger)
	}
	result.FloatBits = typ.FloatBits
	result.Dimension.Base = maps.Clone(typ.Dimension.Base)
	result.UnitSemantics = typ.UnitSemantics
	result.UnitSemantics.SourceFactors = append([]string(nil), typ.UnitSemantics.SourceFactors...)
	result.UnitSemantics.Factors = maps.Clone(typ.UnitSemantics.Factors)
	result.UnitSemantics.Categories = maps.Clone(typ.UnitSemantics.Categories)
	cloneRat := func(value *big.Rat) *big.Rat {
		if value == nil {
			return nil
		}
		return new(big.Rat).Set(value)
	}
	result.UnitSemantics.Scale = cloneRat(typ.UnitSemantics.Scale)
	result.UnitSemantics.Offset = cloneRat(typ.UnitSemantics.Offset)
	result.UnitSemantics.LogBase = cloneRat(typ.UnitSemantics.LogBase)
	result.UnitSemantics.LogFactor = cloneRat(typ.UnitSemantics.LogFactor)
	for index, argument := range typ.TypeArgs {
		result.TypeArgs[index] = shapedAlgebraTypeSnapshot(argument)
	}
	return result
}
