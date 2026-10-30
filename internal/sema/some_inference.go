package sema

import "sec/internal/ast"

// Contextual and self-inferred Some(value) (MD-102). A bare Some(value)
// whose payload has an independently inferable type T has the type
// Option[T] when nothing else supplies one; an expected Option type still
// wins and shapes the payload. As a deferred call argument the payload
// eliminates overload candidates and supplies T for an Option[T] parameter
// exactly like an ordinary argument. Typing inspects payload types only;
// the payload is evaluated once, in the ordinary order.
//
// Rules:
//   - rules/types/types.md — "Contextual and self-inferred `Some(value)`"
//   - rules/declarations/functions.md — § 20 "Overload ranking"
//   - rules/declarations/generics.md — § 18.1 "`Some(value)` payloads (MD-102)"
//   - rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — §§ 11–14

// someIndependentPayloadType returns the type the payload of a bare
// Some(value) has without an expected Option type. A payload that itself
// needs context (None, [], null, or a nested Some of such) has none.
func (a *Analyzer) someIndependentPayloadType(call *ast.CallExpression) (Type, bool) {
	payload := call.Arguments[0]
	if a.isBareNoneArgument(payload) || isEmptyArrayLiteral(payload) || a.isNullSentinel(payload) {
		return Type{}, false
	}
	if inner, ok := payload.(*ast.CallExpression); ok && a.isBareSomeArgument(inner) {
		return a.someIndependentOptionType(inner)
	}
	typ, _ := a.inferCallArgumentExpression(payload)
	if typ.Kind == InvalidType {
		return Type{}, false
	}
	return typ, true
}

// someIndependentOptionType is Option[T] for a bare Some(value) whose
// payload has the independent type T.
func (a *Analyzer) someIndependentOptionType(call *ast.CallExpression) (Type, bool) {
	payload, ok := a.someIndependentPayloadType(call)
	if !ok {
		return Type{}, false
	}
	option, ok := a.types["Option"]
	if !ok {
		return Type{}, false
	}
	option.TypeArgs = []Type{payload}
	return a.instantiateGenericType(option), true
}

// inferStandaloneSome types a bare Some(value) that has no expected type as
// Option[T] from its payload, or reports the missing evidence.
func (a *Analyzer) inferStandaloneSome(expr *ast.CallExpression) (Type, expressionValue, bool) {
	if !a.isBareSomeArgument(expr) {
		return Type{}, expressionValue{}, false
	}
	option, ok := a.someIndependentOptionType(expr)
	if !ok {
		a.addErrorAtToken(expr.Token, "cannot infer the Option type of %s: its payload has no type of its own; give it an expected Option type such as `let value: Option[Option[int]] := %s`", expr.String(), expr.String())
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	typ, value := a.inferExpressionWithExpected(expr, option)
	return typ, value, true
}

// contextualArgumentPayloadRank checks the payload of a deferred bare
// Some(value) argument against the payload type U of an Option[U]
// parameter like an ordinary argument and returns its overload rank. Other
// contextual arguments, and a payload without a type of its own, match by
// family alone.
func (a *Analyzer) contextualArgumentPayloadRank(argument ast.Expression, parameter Type) (int, bool) {
	call, ok := argument.(*ast.CallExpression)
	if !ok || !a.isBareSomeArgument(call) || len(parameter.TypeArgs) != 1 {
		return 0, true
	}
	payloadType, known := a.someIndependentPayloadType(call)
	if !known {
		return 0, true
	}
	expected := parameter.TypeArgs[0]
	if typeMentionsGenericParameter(expected) {
		return 0, true
	}
	payload := call.Arguments[0]
	payloadType = a.contextualCallArgumentType(payload, payloadType, expected)
	if !a.canPassCallArgumentUnrecorded(expected, payloadType, payload) {
		return 0, false
	}
	return overloadArgumentRank(expected, payloadType), true
}

// contextualInferenceArgumentTypes gives each deferred bare Some(value)
// argument the type Option[T] of its payload for generic inference. Other
// deferred arguments keep their placeholder, which inference skips, so
// another argument must supply their type parameters.
func (a *Analyzer) contextualInferenceArgumentTypes(arguments []ast.Expression, types []Type) []Type {
	result := append([]Type(nil), types...)
	for index, argument := range arguments {
		if index >= len(result) || !a.deferredContextualArguments[argument] {
			continue
		}
		call, ok := argument.(*ast.CallExpression)
		if !ok || !a.isBareSomeArgument(call) {
			continue
		}
		if option, known := a.someIndependentOptionType(call); known {
			result[index] = option
		}
	}
	return result
}

// concreteContextualParameterType re-instantiates a generic union parameter
// type such as Option[T] after generic substitution, so its variant payloads
// carry the inferred type arguments (MD-102).
func (a *Analyzer) concreteContextualParameterType(parameter Type) Type {
	if parameter.Kind != UnionType || len(parameter.TypeArgs) == 0 || typeMentionsGenericParameter(parameter) {
		return parameter
	}
	template, ok := a.types[parameter.Name]
	if !ok || template.Kind != UnionType || len(template.GenericParameters) != len(parameter.TypeArgs) {
		return parameter
	}
	template.TypeArgs = parameter.TypeArgs
	return a.instantiateGenericType(template)
}

// isDeferredContextualPlaceholder reports the argument type callArgumentTypes
// gives a deferred contextual argument.
func isDeferredContextualPlaceholder(typ Type) bool {
	return typ.Kind == InvalidType && typ.Name == "contextual"
}
