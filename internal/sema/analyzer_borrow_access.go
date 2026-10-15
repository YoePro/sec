package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// checkBorrowedRead rejects conflicting owner access using canonical Place overlap.
// Rules: rules/memory/borrowing.md — §§4(1–3),6(1–7),22,32(7–8).
func (a *Analyzer) checkBorrowedRead(name string, token lexer.Token) bool {
	place, ok := a.rootPlace(name)
	if !ok {
		return false
	}
	return a.checkBorrowedReadPlace(place, token)
}

// checkBorrowedReadPlace rejects conflicting owner access using canonical Place overlap.
// Rules: rules/memory/borrowing.md — §§4(1–3),6(1–7),22,32(7–8).
func (a *Analyzer) checkBorrowedReadPlace(place Place, token lexer.Token) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if record.Kind != mutableBorrow || record.Holder == candidate.Root || record.Holder == candidate.ReferenceHolder || !borrowRecordMayOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowOwnerReadConflict, "cannot read %s because it may remain mutably borrowed from a previous loop iteration", place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowOwnerReadConflict, "cannot read %s while it is mutably borrowed", place.String())
			return true
		}
	}
	return false
}

// checkBorrowedMutation rejects conflicting owner access using canonical Place overlap.
// Rules: rules/memory/borrowing.md — §§4(1–3),6(1–7),22,32(7–8).
func (a *Analyzer) checkBorrowedMutation(name string, token lexer.Token) bool {
	place, ok := a.rootPlace(name)
	if !ok {
		return false
	}
	return a.checkBorrowedMutationPlace(place, token)
}

// checkBorrowedMutationPlace rejects conflicting owner access using canonical Place overlap.
// Rules: rules/memory/borrowing.md — §§4(1–3),6(1–7),22,32(7–8).
func (a *Analyzer) checkBorrowedMutationPlace(place Place, token lexer.Token) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if isDeferredUseKind(record.Kind) {
				continue
			}
			if candidate.ReferenceHolder != "" && record.Holder == candidate.ReferenceHolder {
				continue
			}
			if !borrowRecordMayOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowMutationConflict, "cannot assign to %s because it may remain borrowed from a previous loop iteration", place.String())
				return true
			}
			if record.Kind == mutableBorrow {
				a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowMutationConflict, "cannot assign to %s while it is mutably borrowed", place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowMutationConflict, "cannot assign to %s while it is shared borrowed", place.String())
			return true
		}
	}
	return false
}

// checkBorrowedMove rejects conflicting owner access using canonical Place overlap.
// Rules: rules/memory/borrowing.md — §§4(1–3),6(1–7),22,32(7–8).
func (a *Analyzer) checkBorrowedMove(name string, token lexer.Token) bool {
	place, ok := a.rootPlace(name)
	if !ok {
		return false
	}
	return a.checkBorrowedMovePlace(place, token)
}

// checkBorrowedMovePlace rejects conflicting owner access using canonical Place overlap.
// Rules: rules/memory/borrowing.md — §§4(1–3),6(1–7),22,32(7–8).
func (a *Analyzer) checkBorrowedMovePlace(place Place, token lexer.Token) bool {
	return a.checkBorrowedMovePlaceForAction(place, token, "move")
}

// checkBorrowedMovePlaceForAction validates ownership-invalidating move or
// discard against deferred Place dependencies and ordinary live borrows.
//
// Rules:
//   - rules/memory/borrowing.md — §21 "Defer and delayed use", §22 "Moves, discard, replacement, and destruction"
func (a *Analyzer) checkBorrowedMovePlaceForAction(place Place, token lexer.Token, action string) bool {
	if a.checkDeferredUsePlace(place, token, action) {
		return true
	}
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if isDeferredUseKind(record.Kind) {
				continue
			}
			if candidate.ReferenceHolder != "" && record.Holder == candidate.ReferenceHolder {
				continue
			}
			if !borrowRecordMayOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowOwnershipEndConflict, "cannot move %s because it may remain borrowed from a previous loop iteration", place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowOwnershipEndConflict, "cannot move %s while it is borrowed", place.String())
			return true
		}
	}
	return false
}
