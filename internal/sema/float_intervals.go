package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema/constant"
)

// floatingConstantValue evaluates pure scalar syntax at its resolved floating
// width and uses only immutable binding values, never a mutable initial value.
// Unit-changing calls are excluded: a numeric coordinate is not a unit conversion.
// Rules: rules/types/types.md — Binary floating-point types, Context shaping;
// rules/foundations/operators.md — Floating arithmetic.
func (a *Analyzer) floatingConstantValue(expr ast.Expression, typ Type) (*big.Rat, bool) {
	if typ.Kind != FloatType || (typ.FloatBits != 32 && typ.FloatBits != 64) {
		return nil, false
	}
	switch node := expr.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols[node.Value]
		if !ok || symbol.Mutable || symbol.Volatile || symbol.FloatingConstant == nil {
			return nil, false
		}
		return new(big.Rat).Set(symbol.FloatingConstant), true
	case *ast.PrefixExpression:
		value, ok := a.floatingConstantValue(node.Right, typ)
		if !ok {
			return nil, false
		}
		if node.Operator == "-" {
			return value.Neg(value), true
		}
		if node.Operator == "+" {
			return value, true
		}
		return nil, false
	case *ast.InfixExpression:
		left, lok := a.floatingConstantValue(node.Left, typ)
		right, rok := a.floatingConstantValue(node.Right, typ)
		if !lok || !rok {
			return nil, false
		}
		// Operand conversions change coordinates before arithmetic evaluates.
		if plan, ok := a.UnitConversionPlanOf(node.Left); ok {
			left.Add(new(big.Rat).Mul(left, plan.Factor), plan.Offset)
		}
		if plan, ok := a.UnitConversionPlanOf(node.Right); ok {
			right.Add(new(big.Rat).Mul(right, plan.Factor), plan.Offset)
		}
		v, outcome := a.compileTimeBinary(node.Operator, DefaultConstant{Kind: FloatType, FloatBits: typ.FloatBits, Exact: left}, DefaultConstant{Kind: FloatType, FloatBits: typ.FloatBits, Exact: right})
		return v.Exact, outcome == compileTimeEvaluated && v.Exact != nil
	}
	value, ok := defaultConstantFromExpression(expr)
	if !ok {
		return nil, false
	}
	converted, outcome := a.prepareCompileTimeConstant(value, typ)
	return converted.Exact, outcome == compileTimeEvaluated && converted.Exact != nil
}

// recordFloatingBindingConstant snapshots an immutable initializer after its
// proven implicit unit conversion. Symbols carry the fact through lexical scope
// copies, so shadowing, mutable values and later function analyses cannot reuse it.
// Rules: rules/types/units.md — Exact fixed conversions;
// rules/foundations/names_scopes_visibility.md — Lexical scopes.
func (a *Analyzer) recordFloatingBindingConstant(name string, expr ast.Expression) {
	symbol, ok := a.symbols[name]
	if !ok || symbol.Mutable || symbol.Volatile || symbol.Type.Kind != FloatType {
		return
	}
	typ := symbol.Type
	if source, ok := a.expressionTypes[expr]; ok && source.Kind == FloatType {
		typ = source
	}
	value, ok := a.floatingConstantValue(expr, typ)
	if !ok {
		return
	}
	if plan, ok := a.UnitConversionPlanOf(expr); ok {
		value.Add(new(big.Rat).Mul(value, plan.Factor), plan.Offset)
	}
	symbol.FloatingConstant = value
	a.symbols[name] = symbol
}

// floatingInterval bounds finite source coordinates using immutable constants,
// named range contracts and non-invalidated dominating comparisons. Both bounds
// must be proven finite; unconstrained infinities and NaN never become a proof.
// Rules: rules/types/units.md — No hidden precision loss;
// rules/types/contracts.md — Range; rules/foundations/operators.md — Floating comparisons.
func (a *Analyzer) floatingInterval(expr ast.Expression, typ Type) (*big.Rat, *big.Rat, bool) {
	if value, ok := a.floatingConstantValue(expr, typ); ok {
		return value, new(big.Rat).Set(value), true
	}
	var lower, upper *big.Rat
	bound := func(value *big.Rat, below, exclusive bool) {
		if value == nil {
			return
		}
		v, ok := constant.FloatEndpoint(value, typ.FloatBits, below, exclusive)
		if !ok {
			return
		}
		r, ok := constant.FloatExact(v, typ.FloatBits)
		if !ok {
			return
		}
		if below {
			if upper == nil || r.Cmp(upper) < 0 {
				upper = r
			}
		} else {
			if lower == nil || r.Cmp(lower) > 0 {
				lower = r
			}
		}
	}
	for _, contract := range typ.Contracts {
		if r, ok := contract.(RangeContract); ok {
			bound(r.ExactMin, false, false)
			bound(r.ExactMax, true, r.Exclusive)
		}
	}
	subject, pure := relationOperandSpelling(expr)
	// Only ordinary scalar binding reads are stable across dominating facts.
	// A repeated getter or volatile read is a new value even without a local write.
	if id, ok := expr.(*ast.Identifier); ok {
		if symbol, ok := a.symbols[id.Value]; !ok || symbol.Volatile {
			pure = false
		}
	} else {
		pure = false
	}
	if pure {
		for _, fact := range a.activeComparisonFacts() {
			op := fact.Operator
			var other ast.Expression
			if spelling, ok := relationOperandSpelling(fact.Left); ok && spelling == subject {
				other = fact.Right
			} else if spelling, ok := relationOperandSpelling(fact.Right); ok && spelling == subject {
				other = fact.Left
				op = mirroredRelation(op)
			} else {
				continue
			}
			// A converted comparison constant must already use the subject coordinates.
			if ot, ok := a.expressionTypes[other]; ok && hasUnitSemantics(ot) {
				factor, offset, ok := unitConversionCoordinates(effectiveUnitSemantics(ot), effectiveUnitSemantics(typ))
				if !ok || factor.Cmp(big.NewRat(1, 1)) != 0 || offset.Sign() != 0 {
					continue
				}
			}
			value, ok := a.floatingConstantValue(other, typ)
			if !ok {
				continue
			}
			switch op {
			case "<=":
				bound(value, true, false)
			case "<":
				bound(value, true, true)
			case ">=":
				bound(value, false, false)
			case ">": // FloatEndpoint has exclusive upper support; negate to select the strict lower endpoint.
				negative := new(big.Rat).Neg(value)
				v, ok := constant.FloatEndpoint(negative, typ.FloatBits, true, true)
				if ok {
					r, _ := constant.FloatExact(v, typ.FloatBits)
					bound(r.Neg(r), false, false)
				}
			case "==":
				bound(value, false, false)
				bound(value, true, false)
			}
		}
	}
	return lower, upper, lower != nil && upper != nil && lower.Cmp(upper) <= 0
}
