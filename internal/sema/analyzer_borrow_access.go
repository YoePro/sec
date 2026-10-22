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
	return a.checkBorrowedReadPlaceForAction(place, token, "read")
}

// checkBorrowedReadPlaceForAction applies the shared-access check of
// checkBorrowedReadPlace to another operation that needs shared authority.
func (a *Analyzer) checkBorrowedReadPlaceForAction(place Place, token lexer.Token, action string) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if record.Kind != mutableBorrow || record.Holder == candidate.Root || a.grantingBorrowAncestor(candidate, record) || !borrowRecordMayOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowOwnerReadConflict, "cannot %s %s because it may remain mutably borrowed from a previous loop iteration", action, place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowOwnerReadConflict, "cannot %s %s while it is mutably borrowed", action, place.String())
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
	return a.checkBorrowedMutationPlaceForAction(place, token, "assign to")
}

// checkBorrowedMutationPlaceForAction checks the same canonical live-borrow
// authority for assignment and storage-invalidating collection operations.
// Rules: rules/memory/borrowing.md — §§6,8.1,12,14.5;
// rules/collections/collections.md — §§6.5,19.
func (a *Analyzer) checkBorrowedMutationPlaceForAction(place Place, token lexer.Token, action string) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if isDeferredUseKind(record.Kind) {
				continue
			}
			if a.grantingBorrowAncestor(candidate, record) {
				continue
			}
			if !borrowRecordMayOverlap(candidate, record) {
				continue
			}
			if record.LoopCarried {
				a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowMutationConflict, "cannot %s %s because it may remain borrowed from a previous loop iteration", action, place.String())
				return true
			}
			if record.Kind == mutableBorrow {
				a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowMutationConflict, "cannot %s %s while it is mutably borrowed", action, place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, diagnostics.BorrowMutationConflict, "cannot %s %s while it is shared borrowed", action, place.String())
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
			if a.grantingBorrowAncestor(candidate, record) {
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
