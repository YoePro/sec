package sema

import "sec/internal/diagnostics"

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
	a.addErrorAtTokenWithMetadata(
		info.PayloadToken,
		diagnostics.UnionPayloadMoveStorage,
		"Move the whole union into a local owning variable before matching, or bind the payload with `ref` (or `ref mut` when mutation is required and permitted).",
		"cannot move payload %s because its union storage is not an independently tracked local owner",
		info.PayloadPlace.String(),
	)
}
