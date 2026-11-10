package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validatePropertyReadReceiver applies the inferred getter capability to the
// actual receiver before publishing an accessor call. Its borrow footprint is
// conservatively the receiver, not a guessed backing field; returned-reference
// dependencies require their own summaries.
// Rules: rules/declarations/properties.md — §§4,8,14;
// rules/memory/borrowing.md — §§5.4,6–8;
// rules/memory/ownership.md — §8.
func (a *Analyzer) validatePropertyReadReceiver(expr ast.Expression, property Property) bool {
	if property.Static || property.GetterCallable == nil {
		return true
	}
	var receiver ast.Expression
	if member, ok := expr.(*ast.MemberExpression); ok {
		receiver = member.Object
	} else {
		receiver = &ast.Identifier{Token: expressionToken(expr), Value: "self"}
	}
	mutable := property.GetterCallable.ReceiverMutable
	place, placed := a.resolvePlace(receiver)
	if placed && place.Type.Kind == ReferenceType && place.Type.Element != nil {
		place = a.canonicalDereferencePlace(receiver, place, place.Type)
	}
	// MutexGuard's protected payload has the guard's exclusive authority;
	// its existing ownership/liveness validation still controls guard access.
	// Rules: rules/concurrency/mutex.md — §§28,42–43.
	guard := placed && isMutexGuardType(dereferenceType(place.Type))
	if mutable && !(placed && (place.Mutable || guard)) && !a.freshTemporaryBorrowSource(receiver) {
		a.addErrorAtTokenWithMetadataAndPrevious(expressionToken(expr), property.Token, diagnostics.PropertyReceiverMutable,
			"This getter mutates or mutably borrows its receiver. Use writable storage or ref mut access; a shared reference cannot grant mutable authority.",
			"property getter %s requires a mutable receiver", property.Name)
		return false
	}
	if placed {
		if mutable {
			if a.checkBorrowedMutationPlaceForAction(place, expressionToken(expr), "invoke mutating property getter on") {
				return false
			}
		} else if a.checkBorrowedReadPlaceForAction(place, expressionToken(expr), "invoke property getter on") {
			return false
		}
	}
	a.recordReceiverReferencePlan(*property.GetterCallable, receiver)
	if mutable {
		a.invalidateMutexAcquisitionsForWrite(receiver)
		if placed {
			a.noteLambdaEnvironmentWrite(place, expressionToken(receiver))
		}
	}
	return true
}
