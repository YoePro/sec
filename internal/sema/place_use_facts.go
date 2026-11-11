package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// UnavailablePart is one moved, discarded or otherwise ended part of an
// owner that is still readable as a whole through its remaining parts.
type UnavailablePart struct {
	Place       string
	At          lexer.Token
	Conditional bool
	Reasons     string
}

// ResolvedPlaceUse is the availability fact Sema established at one accepted
// read of a local Place (copy_move.md § 29(2)): the move that last initialized
// the read root on the analyzed path, the parts of the enclosing owner that are
// unavailable at this read (partial aggregate state), whether the read lies in
// a loop that the root was declared outside of, and the later uses of the same
// root in the same callable in source order. It is a snapshot of the facts
// the availability check consumed; nothing is derived again.
type ResolvedPlaceUse struct {
	Place  Place
	File   string
	Line   int
	Column int
	// Origin is the accepted move whose destination last initialized the
	// root binding on the analyzed path; nil when that initialization was
	// not a move.
	Origin *ResolvedTransferPlan
	// UnavailableParts are the moved parts of the enclosing owner, sorted
	// by Place. Empty means the owner is fully available.
	UnavailableParts []UnavailablePart
	// RepeatedByLoop is set when the read lies inside a loop that starts
	// after the root's declaration, so a move here would also be observed
	// by the next iteration.
	RepeatedByLoop bool
	// LaterUses are the source positions of later accepted reads and
	// transfers of the same root in the same callable, in source order.
	LaterUses []lexer.Token
	callable  CallableID
}

// recordPlaceUse snapshots a read that checkPlaceAvailableForRead accepted.
// Rules: rules/memory/copy_move.md — § 29(1)–(2); rules/memory/ownership.md — §§ 20–21.
func (a *Analyzer) recordPlaceUse(place Place, token lexer.Token) {
	if a == nil || token.Line == 0 || place.RootID == 0 {
		return
	}
	if place.RootKind != PlaceRootLocal && place.RootKind != PlaceRootParameter {
		return
	}
	if a.placeUses == nil {
		a.placeUses = map[sourceTokenKey]ResolvedPlaceUse{}
	}
	use := ResolvedPlaceUse{
		Place: cloneSlicePlanPlace(place), File: token.File, Line: token.Line, Column: token.Column,
		callable: a.currentCallable,
	}
	if origin, ok := a.incomingMoves[place.RootID]; ok {
		if plan, planned := a.resolvedTransferPlans[origin]; planned {
			detached := cloneTransferPlan(plan)
			use.Origin = &detached
		}
	}
	for key, at := range a.moved {
		if strings.HasPrefix(key, place.Root+".") || strings.HasPrefix(key, place.Root+"[") {
			state := a.availabilityStates[key]
			use.UnavailableParts = append(use.UnavailableParts, UnavailablePart{
				Place: key, At: at, Conditional: isConditionallyUnavailable(state), Reasons: state.Reasons,
			})
		}
	}
	sort.Slice(use.UnavailableParts, func(i, j int) bool { return use.UnavailableParts[i].Place < use.UnavailableParts[j].Place })
	for _, start := range a.loopStarts {
		if start.File == place.RootToken.File && tokenBefore(place.RootToken, start) {
			use.RepeatedByLoop = true
			break
		}
	}
	a.placeUses[sourceTokenLocation(token)] = use
}

// noteIncomingTransfer retains, per root binding, the accepted transfer that
// last (re)initialized the whole root; any other whole-root initialization
// clears it.
func (a *Analyzer) noteIncomingTransfer(value ast.Expression, destination Place) {
	if a == nil || destination.RootID == 0 || len(destination.Projections) > 0 {
		return
	}
	if a.incomingMoves == nil {
		a.incomingMoves = map[PlaceRootID]ast.Expression{}
	}
	if plan, ok := a.resolvedTransferPlans[value]; ok && plan.Action == TransferMove && !plan.SourceFresh {
		a.incomingMoves[destination.RootID] = value
		return
	}
	delete(a.incomingMoves, destination.RootID)
}

// enterLoopBody records the start of a loop body for RepeatedByLoop and
// returns the restore function.
func (a *Analyzer) enterLoopBody(start lexer.Token) func() {
	a.loopStarts = append(a.loopStarts, start)
	return func() { a.loopStarts = a.loopStarts[:len(a.loopStarts)-1] }
}

// ResolvedPlaceUseAt returns the availability fact of the accepted read at
// one source position, detached, with its later uses filled in.
// Rules: rules/memory/copy_move.md — § 29(2).
func (a *Analyzer) ResolvedPlaceUseAt(file string, line, column int) (ResolvedPlaceUse, bool) {
	if a == nil {
		return ResolvedPlaceUse{}, false
	}
	use, ok := a.placeUses[sourceTokenKey{File: file, Line: line, Column: column}]
	if !ok {
		return ResolvedPlaceUse{}, false
	}
	use.Place = cloneSlicePlanPlace(use.Place)
	use.UnavailableParts = append([]UnavailablePart(nil), use.UnavailableParts...)
	if use.Origin != nil {
		origin := cloneTransferPlan(*use.Origin)
		use.Origin = &origin
	}
	here := lexer.Token{File: file, Line: line, Column: column}
	later := map[sourceTokenKey]lexer.Token{}
	for key, other := range a.placeUses {
		if other.Place.RootID == use.Place.RootID && other.callable == use.callable && other.File == file && tokenBefore(here, lexer.Token{File: other.File, Line: other.Line, Column: other.Column}) {
			later[key] = lexer.Token{File: other.File, Line: other.Line, Column: other.Column}
		}
	}
	for _, plan := range a.resolvedTransferPlans {
		if plan.SourceFresh || plan.Source.RootID != use.Place.RootID || plan.Destination.Context != use.callable || plan.File != file {
			continue
		}
		at := lexer.Token{File: plan.File, Line: plan.Line, Column: plan.Column}
		if tokenBefore(here, at) {
			later[sourceTokenLocation(at)] = at
		}
	}
	use.LaterUses = use.LaterUses[:0]
	for _, token := range later {
		use.LaterUses = append(use.LaterUses, token)
	}
	sort.Slice(use.LaterUses, func(i, j int) bool { return tokenBefore(use.LaterUses[i], use.LaterUses[j]) })
	return use, true
}
