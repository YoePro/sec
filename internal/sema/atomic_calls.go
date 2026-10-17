package sema

import "sec/internal/ast"

// inferAtomicCall checks the canonical public method spelling, arity and
// value and order arguments on atomic storage, including shared receivers.
// Rules: rules/concurrency/atomics.md — §§12–25,27–30,43–45.
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
	supported, known := atomicOperationApplicability(member.Property.Value, element)
	if !known {
		return Type{}, expressionValue{}, false
	}
	if !supported {
		a.addErrorAtToken(member.Property.Token, "%s is not supported for Atomic[%s]", member.Property.Value, typeDisplayName(element))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(expr.GenericArguments) != 0 {
		a.addErrorAtToken(member.Property.Token, "Atomic.%s does not take type arguments; T belongs to Atomic[T]", member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	switch member.Property.Value {
	case "Load":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.Load", 0, 1) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkAtomicOrderArgument(expr, "Load", 0, false) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "Store":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.Store", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		if !a.checkAtomicOrderArgument(expr, "Store", 1, false) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Swap":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.Swap", 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		if !a.checkAtomicOrderArgument(expr, "Swap", 1, false) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
		if !a.checkCompilerKnownCallArity(expr, "Atomic."+member.Property.Value, 1, 2) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.checkAtomicValueArgument(expr, element, 0)
		if !a.checkAtomicOrderArgument(expr, member.Property.Value, 1, false) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return element, expressionValue{Display: expr.String()}, true
	case "CompareExchange":
		if !a.checkCompilerKnownCallArity(expr, "Atomic.CompareExchange", 2, 4) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) == 3 {
			a.addErrorAtToken(expr.Token, "Atomic.CompareExchange expects 2 or 4 arguments, got 3")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		expectedValid := a.checkAtomicValueArgument(expr, element, 0)
		desiredValid := a.checkAtomicValueArgument(expr, element, 1)
		successOrder, successValid := a.resolveAtomicOrderArgument(expr, "CompareExchange", 2, false)
		failureOrder, failureValid := a.resolveAtomicOrderArgument(expr, "CompareExchange", 3, true)
		if !successValid || !failureValid {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		result := a.intrinsicGenericType("CompareExchangeResult", element)
		if expectedValid && desiredValid {
			a.recordCompareExchange(expr, member.Object, element, result, successOrder, failureOrder)
		}
		return result, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

// checkAtomicValueArgument preserves the contained nominal type and its
// ordinary value contracts at the atomic call boundary.
// Rules: rules/concurrency/atomics.md — §§12,27(2–6),28(5).
func (a *Analyzer) checkAtomicValueArgument(expr *ast.CallExpression, expected Type, index int) bool {
	if index >= len(expr.Arguments) {
		return false
	}
	argType, _ := a.inferExpression(expr.Arguments[index])
	if argType.Kind == InvalidType {
		return false
	}
	if !a.canInitialize(expected, argType, expr.Arguments[index]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[index]), "atomic argument %d must be %s, got %s", index+1, typeDisplayName(expected), typeDisplayName(argType))
		return false
	}
	return true
}
