package sema

// captureReferenceOrigins snapshots the reference-origin type of every
// reference-carrying binding at the end of a branch. Branch analysis restores
// the outer symbol table, so without this snapshot an origin a branch gives
// an outer reference holder (for example a holder rebound to Arena storage)
// would be lost at the join.
func captureReferenceOrigins(symbols map[string]Symbol) map[string]Type {
	origins := map[string]Type{}
	for name, symbol := range symbols {
		if typeCarriesReferenceOrigin(symbol.Type) {
			origins[name] = symbol.Type
		}
	}
	return origins
}

// mergeContinuingReferenceOrigins joins the reference origins of the outer
// bindings over every branch that reaches the join. A branch without a
// snapshot (a fallthrough or omitted branch) keeps the pre-branch origin.
// When branches disagree the join keeps the origin whose later checks are
// strongest: Arena storage first, with the oldest generation for the same
// domain so a Reset between the branch and a later use is still detected,
// then invocation-local storage, then the first continuing origin. A holder
// that may point into an Arena therefore stays subject to stale-generation
// and Reset/Release dependency checks after the join.
//
// Rules:
//   - rules/memory/arena.md — §§36(1–4), 44(1–2) (dependencies across Reset/Release)
//   - rules/corrections/applied/correction25-20260823.md, correction28-20260823.md — corrected branch/switch/loop generation-state propagation
//   - rules/control-flow/flowcontrol_if.md — § 27 "Sema and flow-analysis requirements"
//   - rules/memory/lifetime_analysis.md — control-flow joins
func (a *Analyzer) mergeContinuingReferenceOrigins(branches ...branchAnalysis) {
	for name, symbol := range a.symbols {
		if !typeCarriesReferenceOrigin(symbol.Type) {
			continue
		}
		var chosen *Type
		for _, branch := range branches {
			if !branch.continues {
				continue
			}
			candidate := symbol.Type
			if branch.referenceOrigins != nil {
				if origin, ok := branch.referenceOrigins[name]; ok {
					candidate = origin
				}
			}
			if chosen == nil || strongerReferenceOrigin(candidate, *chosen) {
				value := candidate
				chosen = &value
			}
		}
		if chosen == nil || sameReferenceOriginType(*chosen, symbol.Type) {
			continue
		}
		symbol.Type = referenceTypeWithOrigin(symbol.Type, *chosen)
		a.symbols[name] = symbol
	}
}

func referenceOriginRank(typ Type) int {
	switch {
	case typ.ReferenceOriginStorage == StorageOriginArena && typ.ReferenceOriginName != "":
		return 3
	case typ.ReferenceOriginLocal || typ.ReferenceOriginMatchScoped:
		return 2
	default:
		return 1
	}
}

func strongerReferenceOrigin(candidate, current Type) bool {
	candidateRank, currentRank := referenceOriginRank(candidate), referenceOriginRank(current)
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	return candidateRank == 3 && candidate.ReferenceOriginName == current.ReferenceOriginName &&
		candidate.ReferenceOriginGeneration < current.ReferenceOriginGeneration
}

func sameReferenceOriginType(left, right Type) bool {
	return left.ReferenceOriginName == right.ReferenceOriginName && left.ReferenceOriginLocal == right.ReferenceOriginLocal &&
		left.ReferenceOriginMatchScoped == right.ReferenceOriginMatchScoped && left.ReferenceOriginStorage == right.ReferenceOriginStorage &&
		left.ReferenceOriginGeneration == right.ReferenceOriginGeneration && left.ReferenceOriginToken == right.ReferenceOriginToken
}

// mergeLoopExitReferenceOrigins joins outer reference-holder origins at a
// loop exit: the zero-iteration entry (the restored pre-loop symbols), the end
// of the body (bodySymbols), and every break edge of the loop.
func (a *Analyzer) mergeLoopExitReferenceOrigins(bodySymbols map[string]Symbol, frame loopBreakFrame) {
	branches := []branchAnalysis{{continues: true}, {continues: true, referenceOrigins: captureReferenceOrigins(bodySymbols)}}
	for _, edge := range frame.breaks {
		branches = append(branches, branchAnalysis{continues: true, referenceOrigins: edge.referenceOrigins})
	}
	a.mergeContinuingReferenceOrigins(branches...)
}

// loopHeaderReferenceOrigins joins outer reference-holder origins at a loop
// header over the entry, the end of a body that falls through, and every
// continue edge. Only bindings whose joined origin differs from the entry are
// returned; an empty result means the header origins are already a fixed
// point.
func loopHeaderReferenceOrigins(entry map[string]Symbol, body map[string]Symbol, bodyFallsThrough bool, frame loopBreakFrame) map[string]Type {
	snapshots := []map[string]Type{}
	if bodyFallsThrough {
		snapshots = append(snapshots, captureReferenceOrigins(body))
	}
	for _, edge := range frame.continues {
		snapshots = append(snapshots, edge.referenceOrigins)
	}
	changed := map[string]Type{}
	for name, symbol := range entry {
		if !typeCarriesReferenceOrigin(symbol.Type) {
			continue
		}
		chosen := symbol.Type
		for _, snapshot := range snapshots {
			if origin, ok := snapshot[name]; ok && strongerReferenceOrigin(origin, chosen) {
				chosen = origin
			}
		}
		if !sameReferenceOriginType(chosen, symbol.Type) {
			changed[name] = chosen
		}
	}
	return changed
}
