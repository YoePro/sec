package sema

import "sort"

// ResolvedPartialPlaceUses returns, in source order, the accepted reads at
// which part of the enclosing owner is unavailable (partial aggregate state),
// detached. Tooling lists them from the same facts as the place-use hover.
// Rules: rules/memory/copy_move.md — § 29(1)–(2); rules/compiler/semantic_ir.md — § 109(1)–(3).
func (a *Analyzer) ResolvedPartialPlaceUses() []ResolvedPlaceUse {
	if a == nil {
		return nil
	}
	var uses []ResolvedPlaceUse
	for _, use := range a.placeUses {
		if len(use.UnavailableParts) == 0 {
			continue
		}
		use.Place = cloneSlicePlanPlace(use.Place)
		use.UnavailableParts = append([]UnavailablePart(nil), use.UnavailableParts...)
		use.Origin, use.LaterUses = nil, nil
		uses = append(uses, use)
	}
	sort.Slice(uses, func(i, j int) bool {
		if uses[i].File != uses[j].File {
			return uses[i].File < uses[j].File
		}
		if uses[i].Line != uses[j].Line {
			return uses[i].Line < uses[j].Line
		}
		return uses[i].Column < uses[j].Column
	})
	return uses
}
