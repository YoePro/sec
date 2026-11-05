package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// reportUnionPayloadMoveStorage explains why a by-value match binding cannot
// extract a move-only payload from borrowed, computed, aliased, or otherwise
// externally observable union storage. Such storage cannot retain the partial
// moved state required after transferring only its active payload.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — §13 "Whole-payload ownership modes"
//   - rules/control-flow/flowcontrol_match.md — §14 "Borrowed subjects cannot transfer ownership"
//   - rules/control-flow/flowcontrol_match.md — §18 "Ownership state after match"
//   - rules/memory/ownership.md — §18 "Partial moves"
//   - rules/tooling/diagnostics.md — §2(8) explanation and correction requirements
//   - rules/corrections/applied/diagnostics-ownership-v2-correction-20260826.md — "Required explanation shape"
func (a *Analyzer) reportUnionPayloadMoveStorage(info matchPatternInfo) {
	if info.BorrowedSubject || info.PayloadPlace.RootKind == PlaceRootDeref || placeHasDereference(info.PayloadPlace) {
		// flowcontrol_match.md § 14: a borrowed subject owns nothing to move.
		a.addErrorAtTokenWithMetadata(
			info.PayloadToken,
			diagnostics.UnionPayloadMoveStorage,
			"The subject is only borrowed, so it cannot give up its payload. Bind it with `ref` (or `ref mut` through a ref mut subject), or match an owned value instead.",
			"cannot move payload %s out of a borrowed match subject",
			info.PayloadPlace.String(),
		)
		return
	}
	a.addErrorAtTokenWithMetadata(
		info.PayloadToken,
		diagnostics.UnionPayloadMoveStorage,
		"Move the whole union into a local owning variable before matching, or bind the payload with `ref` (or `ref mut` when mutation is required and permitted).",
		"cannot move payload %s because its union storage is not an independently tracked local owner",
		info.PayloadPlace.String(),
	)
}

// placeHasDereference reports a Place reached through a reference.
func placeHasDereference(place Place) bool {
	for _, projection := range place.Projections {
		if projection.Kind == PlaceDereference {
			return true
		}
	}
	return false
}

// matchSubjectPlace resolves the Place a match observes. A subject reached
// through `ref` or `ref mut` is its referent: payload bindings borrow from it
// and can never move out of it (flowcontrol_match.md § 14).
func (a *Analyzer) matchSubjectPlace(subject ast.Expression) (Place, bool) {
	place, ok := a.resolvePlace(subject)
	if ok && place.Type.Kind == ReferenceType && place.Type.Element != nil {
		element := *place.Type.Element
		place = a.canonicalDereferencePlace(subject, place, place.Type)
		place.Type = element
	}
	return place, ok
}
