package sema

import "sec/internal/ast"

// inferAtomicCall checks the canonical public method spelling, arity and
// value arguments on atomic storage, including shared-reference receivers.
// Rules: rules/concurrency/atomics.md — §§12–13,27–30,45.
func (a *Analyzer) inferAtomicCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiverType, ok := a.expressionTypes[member.Object]
	if !ok {
		receiverType, ok = a.compilerKnownReceiverType(member.Object)
	}
	receiverType = dereferenceType(receiverType)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !isAtomicType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	element := receiverType.TypeArgs[0]
	switch member.Property.Value {
	case "Load":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.Load", 0, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "Store":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.Store", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Swap":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.Swap", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return element, expressionValue{Display: expr.String()}, true
	case "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
		if !a.checkCompilerKnownCallArity(expr, "Atomic."+member.Property.Value, 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !atomicFetchOperationSupported(member.Property.Value, element) {
			a.addErrorAtToken(member.Property.Token, "%s is not supported for Atomic[%s]", member.Property.Value, typeDisplayName(element))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		return element, expressionValue{Display: expr.String()}, true
	case "CompareExchange":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.CompareExchange", 2, 4) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 3 {
			a.addErrorAtToken(expr.Token, "Atomic.CompareExchange expects 2 or 4 arguments, got 3")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		a.checkAtomicValueArgument(expr, element, 1)
		return Type{Name: "CompareExchangeResult", Kind: StructType, TypeArgs: []Type{element}}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

// checkAtomicValueArgument preserves the contained nominal type and its
// ordinary value contracts at the atomic call boundary.
// Rules: rules/concurrency/atomics.md — §§12,27(2–6),28(5).
func (a *Analyzer) checkAtomicValueArgument(expr *ast.CallExpression, expected Type, index int) {
	if index >= len(expr.Arguments) {
		return
	}
	argType, _ := a.inferExpression(expr.Arguments[index])
	if argType.Kind != InvalidType && !a.canInitialize(expected, argType, expr.Arguments[index]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[index]), "atomic argument %d must be %s, got %s", index+1, typeDisplayName(expected), typeDisplayName(argType))
	}
}
