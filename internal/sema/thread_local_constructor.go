package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// inferThreadLocalConstructor validates the initial-value overload after concrete
// payload substitution. It stores a copyable template, not a per-thread value.
// Factory execution and runtime slot initialization remain separate capabilities.
// Rules: rules/concurrency/thread_local.md — §§4(2),7(1–6),20(4).
func (a *Analyzer) inferThreadLocalConstructor(expr *ast.CallExpression) (Type, expressionValue, bool) {
	display := expressionValue{Display: expr.String()}
	invalid := Type{Kind: InvalidType}
	if len(expr.GenericArguments) != 1 {
		a.addErrorAtToken(expr.Token, "ThreadLocal requires exactly 1 value type, got %d", len(expr.GenericArguments))
		return invalid, display, true
	}
	payload, ok := a.resolveType(expr.GenericArguments[0])
	if !ok {
		return invalid, display, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "ThreadLocal expects 1 argument, got %d", len(expr.Arguments))
		return invalid, display, true
	}
	actual, _ := a.inferExpressionWithExpected(expr.Arguments[0], payload)
	if actual.Kind == InvalidType {
		return invalid, display, true
	}
	if actual.Kind == FunctionType && len(actual.FunctionParameterTypes) == 0 && actual.FunctionReturnType != nil && sameConcreteType(*actual.FunctionReturnType, payload) && !a.canInitialize(payload, actual, expr.Arguments[0]) {
		a.reportCapabilityUnavailable(expr.Token, "Initialize the ThreadLocal with a value until lazy first-access factory execution is implemented.", "compiler capability unavailable: ThreadLocal factory initialization requires lazy first-access execution")
		return invalid, display, true
	}
	if !implicitlyCopyable(payload) {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.ThreadLocalInitialNotCopyable,
			"Use a factory initializer for non-copyable payloads; factory initialization is a separate implementation capability.",
			"ThreadLocal[%s](initial) requires an implicitly copyable value type", typeDisplayName(payload))
		return invalid, display, true
	}
	if !a.checkInitializerType(payload, actual, expr.Arguments[0]) || a.checkDeclaredContractExpression(payload, expr.Arguments[0]) {
		return invalid, display, true
	}
	function := Function{Name: "ThreadLocal.init", Parameters: []FunctionParameter{{Name: "initial", Type: payload}}}
	if !a.validateCallArgumentOwnership(function, expr.Arguments, nil, expr) {
		return invalid, display, true
	}
	a.markMovedCallArguments(function, expr.Arguments, nil, false)
	return a.intrinsicGenericType("ThreadLocal", payload), display, true
}
