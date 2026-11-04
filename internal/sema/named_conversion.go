package sema

import "sec/internal/ast"

// ResolvedNamedConversion is Sema's plan for an explicit conversion between a
// named integer type and its base, or between two named types over the same
// carrier (`Meters(1)`, `int(distance)`). The value is unchanged; only its
// nominal identity changes, so lowering needs no runtime check. Conversions
// that change the carrier, involve units or validate contracts keep their own
// plans.
//
// Rules:
//   - rules/types/types.md — Named types, "Explicit conversions"
//   - rules/compiler/semantic_ir.md — §§ 7(4), 59(2)–(3)
type ResolvedNamedConversion struct {
	SourceType Type
	TargetType Type
}

// recordNamedConversion publishes the plan for an accepted explicit conversion
// whose endpoints share one integer carrier and at least one is named.
func (a *Analyzer) recordNamedConversion(call *ast.CallExpression, target, source Type) {
	if call == nil || len(call.Arguments) != 1 || !target.Named && !source.Named {
		return
	}
	if !namedConversionEndpoint(target) || !namedConversionEndpoint(source) {
		return
	}
	targetCarrier, ok := a.namedScalarCarrier(target)
	sourceCarrier, sourceOK := a.namedScalarCarrier(source)
	if !ok || !sourceOK {
		return
	}
	if !sameConcreteType(targetCarrier, sourceCarrier) {
		// An integer constant shapes directly into the carrier; the explicit
		// conversion already validated that it fits (operators.md "Named types").
		if _, constant := a.integerConstantValue(call.Arguments[0]); !constant || source.Named {
			return
		}
		source = targetCarrier
	}
	if a.resolvedNamedConversions == nil {
		a.resolvedNamedConversions = map[*ast.CallExpression]ResolvedNamedConversion{}
	}
	a.resolvedNamedConversions[call] = ResolvedNamedConversion{SourceType: source, TargetType: target}
}

// namedConversionEndpoint admits an integer type without units or contracts.
func namedConversionEndpoint(typ Type) bool {
	return isIntegerType(typ) && typ.Unit == "" && typ.Dimension.IsZero() && len(typ.Contracts) == 0
}

// namedScalarCarrier follows a named type's declared base chain to the
// built-in type it ends in.
func (a *Analyzer) namedScalarCarrier(typ Type) (Type, bool) {
	for depth := 0; typ.Named && depth < 64; depth++ {
		switch {
		case typ.NamedBase != nil:
			typ = *typ.NamedBase
		default:
			base, found := a.types[typ.Underlying]
			if !found || typ.Underlying == typ.Name {
				return Type{}, false
			}
			typ = base
		}
	}
	return typ, !typ.Named
}

// ResolvedNamedConversionOf returns the recorded plan without inference.
func (a *Analyzer) ResolvedNamedConversionOf(call *ast.CallExpression) (ResolvedNamedConversion, bool) {
	if a == nil || call == nil {
		return ResolvedNamedConversion{}, false
	}
	plan, ok := a.resolvedNamedConversions[call]
	return plan, ok
}
