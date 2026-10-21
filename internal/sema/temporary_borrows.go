package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// MD-076 call-lifetime borrows of owned temporaries. A fresh owned value
// passed to a `ref T` / `ref mut T` parameter or used as a borrowed receiver
// is a compiler-materialized Place owned until the end of its full
// expression. A reference returned through that borrow carries the symbolic
// origin root `$temporary:<site>`; it may be used inside the full expression
// but never retained beyond it.
//
// Rules:
//   - rules/memory/borrowing.md — § 5.4 "Call-lifetime borrows of owned temporaries"
//   - rules/memory/lifetime_analysis.md — § 10.2(3)
//   - rules/memory/destruction.md — § 8(7)

const temporaryOriginRootPrefix = "$temporary:"

// freshTemporaryBorrowSource reports an argument or receiver that is a fresh
// owned value rather than an existing Place or reference: borrowing it needs
// a compiler-materialized temporary Place.
func (a *Analyzer) freshTemporaryBorrowSource(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	if _, explicit := expr.(*ast.RefExpression); explicit {
		return false
	}
	if typ, ok := a.expressionTypes[expr]; ok && (typ.Kind == InvalidType || typeCarriesReferenceOrigin(typ)) {
		return false
	}
	if _, ok := a.resolvePlace(expr); ok {
		return false
	}
	switch expr.(type) {
	case *ast.CallExpression, *ast.StructLiteral, *ast.ArrayLiteral:
		return true
	}
	return false
}

// temporaryPlace is the compiler-materialized Place of a fresh argument or
// receiver: its own root and storage identity, no user binding. The borrow
// plan and the returned-reference origin share it, so a reference returned
// through the borrow names the same storage.
func (a *Analyzer) temporaryPlace(expr ast.Expression, typ Type, mutable bool) Place {
	token := expressionToken(expr)
	location := sourceTokenLocation(token)
	site := fmt.Sprintf("%s:%d:%d", location.File, location.Line, location.Column)
	return Place{
		Root:            temporaryOriginRootPrefix + site,
		RootToken:       token,
		RootID:          a.placeRootID(token),
		RootKind:        PlaceRootLocal,
		StorageIdentity: a.placeIdentities.storageIdentity("temporary:" + site),
		Type:            typ,
		Mutable:         mutable,
		Addressable:     true,
	}
}

// temporaryArgumentOrigin is the origin of a borrow of a fresh temporary.
// It is local to the full expression: returning it is a local escape, and
// retaining it is rejected by rejectTemporaryReferenceRetention.
func (a *Analyzer) temporaryArgumentOrigin(expr ast.Expression) localReferenceOrigin {
	place := a.temporaryPlace(expr, a.expressionTypes[expr], false)
	return localOriginWithPlaces(localReferenceOrigin{Name: "temporary " + expr.String(), Token: place.RootToken, Local: true}, []Place{place})
}

// temporaryReferenceOrigin reports a retained value whose reference origin is
// a call-lifetime temporary, directly or through an aggregate or carrier.
func (a *Analyzer) temporaryReferenceOrigin(expr ast.Expression) (localReferenceOrigin, bool) {
	switch expr := expr.(type) {
	case *ast.CallExpression:
		if origin, ok := a.expressionReferenceOrigins[expr]; ok {
			return temporaryOriginIn(origin)
		}
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil && field.Value != nil {
				if origin, ok := a.temporaryReferenceOrigin(field.Value); ok {
					return origin, true
				}
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if origin, ok := a.temporaryReferenceOrigin(element); ok {
				return origin, true
			}
		}
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.temporaryReferenceOrigin(expr.Value)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			return a.temporaryReferenceOrigin(expr.Value)
		}
	case *ast.ConversionExpression:
		return a.temporaryReferenceOrigin(expr.Value)
	}
	return localReferenceOrigin{}, false
}

func temporaryOriginIn(origin localReferenceOrigin) (localReferenceOrigin, bool) {
	for _, place := range localOriginPlaces(origin) {
		if strings.HasPrefix(place.Root, temporaryOriginRootPrefix) {
			return origin, true
		}
	}
	for _, child := range origin.Contained {
		if found, ok := temporaryOriginIn(child); ok {
			return found, true
		}
	}
	return localReferenceOrigin{}, false
}

// rejectTemporaryReferenceRetention rejects binding or storing a reference
// that depends on a temporary cleaned at the end of its full expression
// (MD-076 § 5(4), § 7(3); MD076-N03/N05).
func (a *Analyzer) rejectTemporaryReferenceRetention(value ast.Expression) bool {
	origin, ok := a.temporaryReferenceOrigin(value)
	if !ok {
		return false
	}
	temporary := strings.TrimPrefix(origin.Name, "temporary ")
	a.addErrorAtTokenWithMetadataAndPrevious(
		expressionToken(value),
		origin.Token,
		diagnostics.EscapeLocalStorage,
		"Store "+temporary+" in a local binding and pass that local, or have the call return an owned value; a borrowed temporary lives only until the end of its statement.",
		"cannot keep reference returned by %s: it points into temporary %s, which is cleaned at the end of this statement",
		value.String(),
		temporary,
	)
	return true
}

// recordTemporaryBorrowPlan publishes the implicit call-site borrow of a
// fresh owned argument or receiver: its compiler-materialized temporary Place
// (own root and storage identity, no user binding) and the borrow's facts.
// Lowering materializes the Place from this plan without re-deriving it.
func (a *Analyzer) recordTemporaryBorrowPlan(expr ast.Expression, mutable bool) {
	if a.summaryPass || !a.freshTemporaryBorrowSource(expr) {
		return
	}
	typ, ok := a.expressionTypes[expr]
	if !ok {
		return
	}
	place := a.temporaryPlace(expr, typ, mutable)
	token := place.RootToken
	a.recordReferencePlan(expr, place, mutable, true, token)
	plan := a.resolvedReferencePlans[expr]
	plan.Temporary = true
	a.resolvedReferencePlans[expr] = plan
}
