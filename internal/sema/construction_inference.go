package sema

import "sec/internal/ast"

// inferNewExpression resolves lifecycle construction through the registered init
// overloads and publishes declaration previews before checking arguments.
// Rules: rules/declarations/impl.md — §§15–16;
// rules/tooling/lsp.md — §§4.1(22),13.
func (a *Analyzer) inferNewExpression(expr *ast.NewExpression, handled bool) (Type, expressionValue) {
	result := expressionValue{Display: expr.String()}
	if expr.Type == nil {
		return Type{Kind: InvalidType}, result
	}
	target, ok := a.resolveType(expr.Type)
	if !ok || target.Kind == InvalidType {
		return Type{Kind: InvalidType}, result
	}
	if target.Kind == InterfaceType {
		a.addErrorAtToken(expr.Type.Token, "cannot construct interface type %s with new", typeDisplayName(target))
		return Type{Kind: InvalidType}, result
	}

	if typ, handledMetadata := a.inferShapedMetadataNew(expr, target, handled); handledMetadata {
		return typ, result
	}

	key := initFunctionName(target.Name)
	initializers := a.constructionCandidates(target, a.functions[key])
	a.recordConstructionSignatures(expr, target, initializers)
	hasMatchingArity := false
	for _, initializer := range initializers {
		if len(initializer.Parameters) == len(expr.Arguments) {
			hasMatchingArity = true
			break
		}
	}
	if len(initializers) == 0 || (len(expr.Arguments) == 0 && !hasMatchingArity && IsDefaultable(target)) {
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "new %s has no init overload accepting %d arguments", typeDisplayName(target), len(expr.Arguments))
			return Type{Kind: InvalidType}, result
		}
		if !IsDefaultable(target) {
			a.addErrorAtToken(expr.Token, "new %s() has no explicit init and %s has no valid implicit construction path", typeDisplayName(target), typeDisplayName(target))
			return Type{Kind: InvalidType}, result
		}
		a.resolvedConstructions[expr] = ResolvedConstruction{Target: target, Implicit: true}
		if definition, exists := a.typeDefinitionTokens[target.Name]; exists {
			a.bindDefinition(expr.Type.Token, definition)
		}
		return target, result
	}

	// Bind lifecycle navigation to the `new` keyword. The following type token
	// remains a type occurrence instead of being misclassified as a method.
	callee := &ast.Identifier{Token: expr.Token, Value: key}
	call := &ast.CallExpression{Token: expr.Token, Callee: callee, Function: callee, Arguments: expr.Arguments}
	_, _ = a.inferCallExpressionWithConstruction(call, &target)
	resolved, ok := a.resolvedCalls[call]
	if !ok || !resolved.Function.Initializer || resolved.Function.ConstructionType == nil {
		return Type{Kind: InvalidType}, result
	}
	selected := resolved.Function
	target = *selected.ConstructionType
	a.resolvedConstructions[expr] = ResolvedConstruction{
		Initializer: selected,
		Target:      target,
		ErrorType:   selected.ConstructionError,
	}
	if selected.ConstructionError == nil {
		return target, result
	}
	if !handled {
		a.addErrorAtToken(expr.Token, "new %s selects a fallible init with construction error %s; use try or handle the error locally", typeDisplayName(target), typeDisplayName(*selected.ConstructionError))
		return target, result
	}
	return Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{target, *selected.ConstructionError}}, result
}
