package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// parameterPlace projects a callable parameter into the shared canonical Place
// model by binding identity, preserving exact constant indexes in interprocedural
// summaries. Name fallback is reserved for the compiler-owned implicit receiver.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §13 "Constant index representation"
//   - rules/memory/references.md — §28(4) provenance/projection tests
//   - rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses"
//   - rules/analysis/closure_analysis.md — "Abstract closure identity"
func (b *parameterUsageBuilder) parameterPlace(expression ast.Expression) (*ParameterUsageParameterSummary, Place, bool) {
	switch expression := expression.(type) {
	case *ast.Identifier:
		if resolved, ok := b.analyzer.ResolvedBindingOf(expression); ok {
			if parameter := b.byBinding[resolved.ID]; parameter != nil {
				return parameter, b.parameterRootPlace(parameter, resolvedToken(b.analyzer, resolved.ID), semanticSnapshotType(resolved.Type)), true
			}
			if resolved.Kind == BindingParameter {
				if parameter := b.byName[resolved.Name]; parameter != nil && parameter.Receiver {
					return parameter, b.parameterRootPlace(parameter, expression.Token, semanticSnapshotType(resolved.Type)), true
				}
			}
			// A binding from another callable is not this parameter.
			return nil, Place{}, false
		}
		if parameter := b.byName[expression.Value]; parameter != nil && parameter.Receiver {
			return parameter, b.parameterRootPlace(parameter, expression.Token, parameter.DeclaredType), true
		}
		return nil, Place{}, false
	case *ast.MemberExpression:
		parameter, place, ok := b.parameterPlace(expression.Object)
		if !ok || expression.Property == nil {
			return nil, Place{}, false
		}
		place = appendPlaceProjection(place, PlaceProjection{Kind: PlaceField, Name: expression.Property.Value, Token: expression.Property.Token})
		return parameter, place, true
	case *ast.IndexExpression:
		parameter, place, ok := b.parameterPlace(expression.Left)
		if !ok {
			return nil, Place{}, false
		}
		return parameter, appendPlaceProjection(place, b.analyzer.indexPlaceProjection(expression)), true
	case *ast.SliceExpression:
		parameter, place, ok := b.parameterPlace(expression.Left)
		if !ok {
			return nil, Place{}, false
		}
		projection := PlaceProjection{Kind: PlaceSlice, SliceStartKnown: expression.Start == nil, Token: expression.Token}
		if value, constant := constantIntegerValue(expression.Start); constant && value.IsInt64() {
			projection.SliceStart, projection.SliceStartKnown = value.Int64(), true
		}
		if value, constant := constantIntegerValue(expression.End); constant && value.IsInt64() {
			projection.SliceEnd, projection.SliceEndKnown = value.Int64(), true
			if !expression.Exclusive {
				projection.SliceEnd++
			}
		}
		return parameter, appendPlaceProjection(place, projection), true
	case *ast.RefExpression:
		return b.parameterPlace(expression.Value)
	default:
		return nil, Place{}, false
	}
}

// parameterRootPlace builds the root Place of a summarized parameter with the
// same compiler-owned root identity Sema assigns to that binding: the identity
// follows the parameter declaration (the receiver's summary declaration for
// the implicit receiver), never the use spelling, so interprocedural summary
// Places compare by identity like every other Place.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §10 "Stable place root identity"
//   - rules/compiler/semantic_ir.md — § 10(7)
func (b *parameterUsageBuilder) parameterRootPlace(parameter *ParameterUsageParameterSummary, use lexer.Token, typ Type) Place {
	declaration := parameter.Declaration
	if declaration.Line == 0 {
		declaration = use
	}
	kind := PlaceRootParameter
	if parameter.Receiver {
		kind = PlaceRootReceiver
	}
	return Place{Root: parameter.Name, RootToken: declaration, RootID: b.analyzer.placeRootID(declaration), RootKind: kind, Type: typ}
}
