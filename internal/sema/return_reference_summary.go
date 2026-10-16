package sema

import (
	"strconv"
	"strings"

	"sec/internal/ast"
)

// ReturnReferenceOriginKind classifies the external root a returned
// reference may refer to.
type ReturnReferenceOriginKind string

const (
	ReturnOriginParameter ReturnReferenceOriginKind = "parameter"
	ReturnOriginReceiver  ReturnReferenceOriginKind = "receiver"
	ReturnOriginStatic    ReturnReferenceOriginKind = "static"
)

// ReturnReferenceOrigin is one possible origin of a function's returned
// reference: a parameter (by index), the receiver or static storage, and the
// canonical projection path below that root's referent.
type ReturnReferenceOrigin struct {
	Kind      ReturnReferenceOriginKind
	Parameter int
	// Static names the module-level static binding of a static origin.
	Static      string
	Projections []PlaceProjection
}

// ReturnReferenceSummary is the read-only returned-reference origin summary
// of one function (package15.md §§ 61-62; semantic_ir.md § 45(12)): the
// finite set of possible origins, or Unknown when Sema could not establish
// it. Roots are positional (parameter index, receiver), never source names.
type ReturnReferenceSummary struct {
	Origins []ReturnReferenceOrigin
	Unknown bool
}

// ReturnReferenceSummaryOf returns the returned-reference origin summary of
// function, or false when its result carries no reference.
func (a *Analyzer) ReturnReferenceSummaryOf(function Function) (ReturnReferenceSummary, bool) {
	if !function.HasReturnOrigin || !typeCarriesReferenceOrigin(function.ReturnType) {
		return ReturnReferenceSummary{}, false
	}
	origin := function.ReturnOrigin
	summary := ReturnReferenceSummary{Unknown: origin.Unknown || len(origin.Contained) != 0}
	for _, place := range localOriginPlaces(origin) {
		resolved := ReturnReferenceOrigin{Projections: append([]PlaceProjection(nil), place.Projections...)}
		switch {
		case strings.HasPrefix(place.Root, "$param:"):
			index, err := strconv.Atoi(strings.TrimPrefix(place.Root, "$param:"))
			if err != nil {
				summary.Unknown = true
				continue
			}
			resolved.Kind, resolved.Parameter = ReturnOriginParameter, index
		case place.Root == "$receiver":
			resolved.Kind = ReturnOriginReceiver
		case strings.HasPrefix(place.Root, "$static:"):
			resolved.Kind, resolved.Static = ReturnOriginStatic, strings.TrimPrefix(place.Root, "$static:")
		default:
			summary.Unknown = true
			continue
		}
		summary.Origins = append(summary.Origins, resolved)
	}
	if len(summary.Origins) == 0 {
		summary.Unknown = true
	}
	return summary, true
}

// recordCallReferenceFacts publishes the caller-side facts of a direct
// reference returned by a call, instantiated from the callee's
// returned-origin summary over the actual arguments (package15.md § 62).
func (a *Analyzer) recordCallReferenceFacts(call *ast.CallExpression, function Function, origin localReferenceOrigin) {
	if a.summaryPass || function.ReturnType.Kind != ReferenceType || origin.Unknown {
		return
	}
	place, ok := placeFromReferenceOrigin(origin, "")
	if !ok {
		return
	}
	if a.resolvedCallReferences == nil {
		a.resolvedCallReferences = map[*ast.CallExpression]ReferenceFacts{}
	}
	a.resolvedCallReferences[call] = a.referenceFactsForPlace(place, function.ReturnType.ReferenceMutable, call.Token)
}

// ResolvedCallReferenceOf returns the caller-side facts of the direct
// reference a call returns, instantiated from the callee's returned-origin
// summary; false when the call returns no reference or its origin is unknown.
func (a *Analyzer) ResolvedCallReferenceOf(call *ast.CallExpression) (ReferenceFacts, bool) {
	facts, ok := a.resolvedCallReferences[call]
	return facts, ok
}
