package sema

import "sec/internal/ast"

// mutexGuardMemberOwner selects protected-T metadata only after canonical
// guard members have been checked. It never converts a guard value into T.
// Rules: rules/concurrency/mutex.md — §§30–32.
func mutexGuardMemberOwner(typ Type) Type {
	if isMutexGuardType(typ) {
		return typ.TypeArgs[0]
	}
	return typ
}

// mutexGuardReceiverType exposes the protected receiver to ordinary overload
// and generic resolution, preserving the mutability of a borrowed guard.
// The receiver expression and symbol still identify the guard lifetime.
// Rules: rules/concurrency/mutex.md — §§29,33–35.
func mutexGuardReceiverType(typ Type) Type {
	if typ.Kind == ReferenceType && typ.Element != nil && isMutexGuardType(*typ.Element) {
		protected := mutexGuardMemberOwner(*typ.Element)
		typ.Element = &protected
		return typ
	}
	return mutexGuardMemberOwner(typ)
}

// methodCallName resolves an ordinary method through protected T while retaining
// the actual guard expression for receiver authority and borrow provenance.
// Rules: rules/concurrency/mutex.md — §§30–35;
// rules/declarations/functions.md — §§22,24.
func (a *Analyzer) methodCallName(expr *ast.CallExpression) (string, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return "", false
	}
	if a.expressionNamesType(member.Object) {
		return "", false
	}
	objectType, _ := a.inferExpression(member.Object)
	if objectType.Kind == InvalidType || objectType.Name == "" {
		return "", false
	}
	objectType = mutexGuardMemberOwner(dereferenceType(objectType))
	return objectType.Name + "." + member.Property.Value, true
}

// methodCallReceiver resolves an ordinary method through protected T while retaining
// the actual guard expression for receiver authority and borrow provenance.
// Rules: rules/concurrency/mutex.md — §§30–35;
// rules/declarations/functions.md — §§22,24.
func (a *Analyzer) methodCallReceiver(expr *ast.CallExpression) (methodReceiverInfo, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return methodReceiverInfo{}, false
	}
	if a.expressionNamesType(member.Object) {
		return methodReceiverInfo{}, false
	}
	receiverType, _ := a.inferExpression(member.Object)
	receiverType = mutexGuardReceiverType(receiverType)
	info := methodReceiverInfo{Type: receiverType, Expr: member.Object}
	if symbol, ok := a.symbolForMemberObject(member.Object); ok {
		info.Symbol = &symbol
	}
	return info, receiverType.Kind != InvalidType
}

// mutexGuardMemberReference bounds a reference-valued protected field or getter
// by the actual guard/guard borrow, including computed getter results which do
// not denote addressable stored property Places. Ordinary borrow holders and
// escape summaries then keep the guard alive for every use of that reference.
// Rules: rules/concurrency/mutex.md — §§29(4),32,35;
// rules/memory/borrowing.md — §§5.2(5–7),23.
func (a *Analyzer) mutexGuardMemberReference(expr *ast.MemberExpression, result Type) Type {
	receiver, known := a.expressionTypes[expr.Object]
	if !known || !isMutexGuardType(dereferenceType(receiver)) || !typeCarriesReferenceOrigin(result) {
		return result
	}
	origin, ok := a.actualOriginForSummaryRoot("$receiver", nil, expr.Object)
	if !ok {
		return result
	}
	origin.Mutable = result.ReferenceMutable
	a.expressionReferenceOrigins[expr] = origin
	return a.referenceTypeWithOriginFromExpression(result, expr.Object)
}
