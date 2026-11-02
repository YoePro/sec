package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// inferImplicitImplProperty resolves an unqualified property only when the
// current callable has the required receiver. Static bodies retain their type
// scope for associated declarations, but that scope grants no instance.
// Rules: rules/declarations/static.md — §§9,9.1;
// rules/declarations/properties.md — §§4,8,10.
func (a *Analyzer) inferImplicitImplProperty(expr *ast.Identifier) (Type, bool) {
	if a.currentImplTarget == "" {
		return Type{}, false
	}
	target, ok := a.types[a.currentImplTarget]
	if !ok {
		return Type{}, false
	}
	property, found := a.lookupResolvedProperty(target, expr.Value)
	if !found {
		return Type{}, false
	}
	if a.currentFunctionMetadata.Static && !property.Static {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.StaticInstanceReceiverRequired,
			"Access the property through an instance passed to this function, or use an instance method when self is required.",
			"instance property %s requires a value receiver in static method %s", expr.Value, a.currentFunctionMetadata.Name)
		return Type{Kind: InvalidType}, true
	}
	property, ok = a.resolveReadableProperty(target, expr.Value, expr.Token)
	if !ok {
		return Type{Kind: InvalidType}, true
	}
	a.recordResolvedPropertyRead(expr, target, property)
	return property.Type, true
}

// staticReceiverlessCandidates prevents an enclosing impl scope from supplying
// an implicit receiver to a static function. Explicit value receivers bypass
// this filter; module functions and static overloads remain candidates.
// Rules: rules/declarations/static.md — §§9,9.1;
// rules/declarations/functions.md — §§22–23 (instance/static methods),19–20 (overloads).
func (a *Analyzer) staticReceiverlessCandidates(call *ast.CallExpression, functions []Function) []Function {
	if !a.currentFunctionMetadata.Static {
		return functions
	}
	allowed := make([]Function, 0, len(functions))
	for _, function := range functions {
		if !functionUsesReceiver(function) {
			allowed = append(allowed, function)
		}
	}
	if len(functions) > 0 && len(allowed) == 0 {
		a.addErrorAtTokenWithMetadata(callCalleeDefinitionToken(call), diagnostics.StaticInstanceReceiverRequired,
			"Call the method through an instance passed to this function, or use an instance method when self is required.",
			"method %s requires an instance receiver in static method %s", callExpressionName(call), a.currentFunctionMetadata.Name)
	}
	return allowed
}
