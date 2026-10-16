package sema

import "sec/internal/ast"

// inferMutexCall resolves the exact canonical acquisition overloads, preserving
// duration conversion and Context borrowing without retaining the Context.
// Rules: rules/concurrency/mutex.md — §§3,11–12,20–24,42–43;
// rules/concurrency/blocking.md — "Mutex acquisition";
// rules/corrections/applied/mutex-v2-cross-rulebook-correction-20260907.md — §§1,6–7.
func (a *Analyzer) inferMutexCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
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
	if !isMutexType(receiverType) {
		return Type{}, expressionValue{}, false
	}
	switch member.Property.Value {
	case "Lock":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.Lock does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) > 1 {
			a.addErrorAtToken(expr.Token, "Mutex.Lock expects 0 or 1 arguments, got %d; no timeout/context combination overload exists", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		result := mutexGuardType(receiverType.TypeArgs[0])
		if len(expr.Arguments) == 1 {
			var valid bool
			result, valid = a.resolveMutexLockArgument(expr, result)
			if !valid {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
		}
		// rules/concurrency/blocking.md — "Mutex acquisition": waiting for the
		// lock may block; TryLock never waits.
		a.recordBlockingOperation("Mutex.Lock", member.Property.Token)
		a.checkMutexReacquisition(expr, member)
		a.recordMutexAcquisitionMember(member, receiverType, result)
		return result, expressionValue{Display: expr.String()}, true
	case "TryLock":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.TryLock does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Mutex.TryLock expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		guard := mutexGuardType(receiverType.TypeArgs[0])
		result := a.intrinsicGenericType("Option", guard)
		a.checkMutexReacquisition(expr, member)
		a.recordMutexAcquisitionMember(member, receiverType, result)
		return result, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

// resolveMutexLockArgument selects duration or shared Context by canonical
// nominal identity. Only quantity literals receive contextual duration sugar;
// a Context argument is borrowed for the call, never consumed or retained.
// Rules: rules/concurrency/mutex.md — §§11–12,20(5–6),23–24;
// rules/types/temporal.md — §5.5; rules/memory/borrowing.md — §15.2.
func (a *Analyzer) resolveMutexLockArgument(expr *ast.CallExpression, guard Type) (Type, bool) {
	argument := expr.Arguments[0]
	typ, _ := a.inferExpression(argument)
	if typ.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}
	duration := a.types["duration"]
	if a.canInitialize(duration, typ, argument) {
		return a.intrinsicGenericType("Option", guard), true
	}
	context := a.types["Context"]
	parameter := compilerKnownSharedReference(context)
	compatible := a.canInitialize(parameter, typ, argument)
	// A mutable reference can grant a temporary shared reborrow; that does
	// not transfer the reference holder or permit mutation through this call.
	// Rules: rules/memory/borrowing.md — "Reborrowing".
	if typ.Kind == ReferenceType && typ.Element != nil && sameConcreteType(*typ.Element, context) {
		compatible = true
	}
	if compatible {
		result := compilerKnownResult(guard, a.types["ContextError"])
		callee := Function{Name: "Mutex.Lock", Parameters: []FunctionParameter{{Name: "context", Type: parameter, Ref: true}}, ReturnType: result}
		if !a.validateCallArgumentBorrows(callee, expr.Arguments, nil) {
			return Type{Kind: InvalidType}, false
		}
		a.recordCallReferencePlans(callee, expr.Arguments, nil)
		return result, true
	}
	reason := a.durationConversionRejection(duration, typ, argument)
	if reason != "" {
		a.addErrorAtToken(expressionToken(argument), "Mutex.Lock argument must be duration or ref Context, got %s: %s", typeDisplayName(typ), reason)
	} else {
		a.addErrorAtToken(expressionToken(argument), "Mutex.Lock argument must be duration or ref Context, got %s", typeDisplayName(typ))
	}
	return Type{Kind: InvalidType}, false
}

// recordMutexAcquisitionMember retains the selected overload's actual result
// in shared compiler facts instead of leaving the basic Lock result in tooling.
// Rules: rules/concurrency/mutex.md — §§20–24,61–62.
func (a *Analyzer) recordMutexAcquisitionMember(expression *ast.MemberExpression, receiver, result Type) {
	member, ok := compilerKnownMember(receiver, expression.Property.Value, false)
	if !ok {
		return
	}
	member.Result = result
	a.compilerKnownMemberFacts[sourceTokenLocation(expression.Property.Token)] = member
}
