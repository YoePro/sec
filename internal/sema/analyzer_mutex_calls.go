package sema

import "sec/internal/ast"

// inferMutexCall checks the represented legacy mutex calls by their declared
// argument shape, without recognizing any quantity through its unit spelling.
// Canonical timeout sugar remains unavailable until MD-015 defines the shared
// temporal/unit conversion contract; this path must not invent that conversion.
// Rules: rules/concurrency/mutex.md — §§11,12(2)-(6),20,23;
// rules/concurrency/blocking.md — Mutex acquisition;
// rules/corrections/applied/mutex-v2-cross-rulebook-correction-20260907.md — §§6,7.
func (a *Analyzer) inferMutexCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiverType, ok := a.compilerKnownReceiverType(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if !isMutexType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	switch member.Property.Value {
	case "lock":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.lock does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.lock expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		// rules/concurrency/blocking.md — "Mutex acquisition": waiting for the
		// lock may block; tryLock never waits.
		a.recordBlockingOperation("Mutex.lock", member.Property.Token)
		return mutexGuardType(receiverType.TypeArgs[0]), expressionValue{Display: expr.String()}, true
	case "tryLock":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.tryLock does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.tryLock expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		guard := mutexGuardType(receiverType.TypeArgs[0])
		return Type{Name: "Option", Kind: UnionType, TypeArgs: []Type{guard}}, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}
