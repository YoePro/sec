package sema

import (
	"fmt"

	"sec/internal/ast"
)

// recordExpressionReferencePass publishes a selected shared call's reborrow
// of a mutable slice expression using the referent origin already established
// by Sema. Its pass identity is distinct from the expression's own creation
// or returned-reference identity, even though both have the same source site.
// Unknown origins remain unrepresented; an owned temporary is handled by the
// existing materialization path instead.
// Rules: rules/memory/borrowing.md — §15.2(4), §15.3, §15.4;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§33–37,64–67.
func (a *Analyzer) recordExpressionReferencePass(arg ast.Expression, mutable bool) bool {
	typ, typed := a.expressionTypes[arg]
	if mutable || !typed || typ.Kind != ReferenceType || !typ.ReferenceMutable || typ.Element == nil || typ.Element.Kind != SliceType {
		return false
	}
	place, known := a.referencePlaceOrigin(arg)
	if !known {
		return false
	}
	site := expressionToken(arg)
	facts := a.referenceFactsForPlace(place, mutable, site)
	key := fmt.Sprintf("call-pass:%s:%d:%d", site.File, site.Line, site.Column)
	facts.Borrow = a.placeIdentities.borrow("borrow:" + key)
	facts.Lifetime = a.placeIdentities.lifetime("lifetime:" + key)
	if a.resolvedReferenceUses == nil {
		a.resolvedReferenceUses = map[ast.Expression]ResolvedReferenceUse{}
	}
	use := ResolvedReferenceUse{Kind: ReferenceUsePass, Place: clonePlace(place), Facts: facts, Source: site}
	a.resolvedReferenceUses[arg] = use
	a.recordReferenceValidationEffect(use)
	return true
}

// sharedReferenceCallReborrow permits only an authority reduction at a call:
// ref mut T may grant ref T with the identical nominal referent type.
// It does not change initialization/assignment compatibility, convert owning
// arrays to slices, or authorize the borrow before argument-overlap checks.
// Rules: rules/memory/borrowing.md — §15.3(2), §15.4(1)–(5);
// rules/mlir/packages/sec-mlir-dialect_package15.md — §§59–63;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,64–65.
func sharedReferenceCallReborrow(parameter, argument Type) bool {
	return parameter.Kind == ReferenceType && argument.Kind == ReferenceType &&
		!parameter.Named && !argument.Named &&
		!parameter.ReferenceMutable && argument.ReferenceMutable &&
		parameter.Element != nil && argument.Element != nil &&
		sameConcreteType(*parameter.Element, *argument.Element)
}

// canPassCallArgumentUnrecorded checks candidate compatibility without
// committing a borrow or rewriting the argument type. Keeping the original
// type lets overload ranking prefer an exact mutable parameter. The selected
// call still goes through validateCallArgumentBorrows and publishes its pass.
// Rules: rules/memory/borrowing.md — §15.2(4), §15.3(2), §15.4;
// rules/declarations/functions.md — §37 overload resolution.
func (a *Analyzer) canPassCallArgumentUnrecorded(parameter, argument Type, expr ast.Expression) bool {
	return sharedReferenceCallReborrow(parameter, argument) || a.canInitializeUnrecorded(parameter, argument, expr)
}
