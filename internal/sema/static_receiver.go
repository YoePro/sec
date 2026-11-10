package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
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
	if !a.recordResolvedPropertyRead(expr, target, property) {
		return Type{Kind: InvalidType}, true
	}
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

// reportInstanceQualifiedStatic rejects a static member reached through a
// value: static members are accessed through their type so shared type-level
// state stays explicit.
// Rules: rules/declarations/static.md — § 8 "Static member access", § 28.
func (a *Analyzer) reportInstanceQualifiedStatic(token lexer.Token, owner Type, member string) {
	ownerName := typeDisplayName(owner)
	a.addErrorAtTokenWithMetadata(token, diagnostics.StaticTypeQualifiedAccess,
		"Write "+ownerName+"."+member+" instead of reaching the static member through a value.",
		"static member %s must be accessed through type %s", member, ownerName)
}

// instanceQualifiedStaticStorage reports whether member names type-owned
// static storage of owner, which a value receiver must not reach.
func (a *Analyzer) instanceQualifiedStaticStorage(owner Type, member string) bool {
	if owner.Name == "" {
		return false
	}
	symbol, ok := a.symbols.lookup(owner.Name + "." + member)
	return ok && symbol.Storage == StorageOriginStatic
}

// allStaticFunctions reports a non-empty candidate set whose every member is
// a static fn, which an explicit value receiver must not call.
func allStaticFunctions(functions []Function) bool {
	for _, function := range functions {
		if !function.Static {
			return false
		}
	}
	return len(functions) > 0
}
