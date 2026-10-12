package sema

import (
	"fmt"

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
				a.addBorrowAuthorityConflict(place, token, record, "cannot read %s because it may remain mutably borrowed from a previous loop iteration", place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, "cannot read %s while it is mutably borrowed", place.String())
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
				a.addBorrowAuthorityConflict(place, token, record, "cannot assign to %s because it may remain borrowed from a previous loop iteration", place.String())
				return true
			}
			if record.Kind == mutableBorrow {
				a.addBorrowAuthorityConflict(place, token, record, "cannot assign to %s while it is mutably borrowed", place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, "cannot assign to %s while it is shared borrowed", place.String())
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
				a.addBorrowAuthorityConflict(place, token, record, "cannot move %s because it may remain borrowed from a previous loop iteration", place.String())
				return true
			}
			a.addBorrowAuthorityConflict(place, token, record, "cannot move %s while it is borrowed", place.String())
			return true
		}
	}
	return false
}

// addBorrowAuthorityConflict explains ownership presence independently from
// borrow authority, using the current compiler-owned availability state. It
// retains the owning message and both conflict/origin source locations, and
// explains overlap using the canonical relationship rather than a new checker.
// Rules: rules/memory/borrowing.md — §§6(1–7),32(2–4,6–8),33(1,3);
// rules/tooling/diagnostics.md — §§9,17.
func (a *Analyzer) addBorrowAuthorityConflict(place Place, token lexer.Token, record borrowRecord, format string, args ...any) {
	help := "Ownership availability and borrow access authority are separate; `is available` does not test whether a conflicting borrow is active."
	if known, available := a.currentPlaceAvailability(place); known && available {
		borrow := "an active borrow blocks this access"
		if record.LoopCarried {
			borrow = "a borrow from a previous loop iteration may still block this access"
		}
		help = fmt.Sprintf("%s is available (still owned), but %s. `is available` proves ownership presence, not borrow access authority.", place.String(), borrow)
	}
	help += " " + borrowOverlapExplanation(place, record)
	help += " Use the granting reference where permitted, or end the conflicting borrow before accessing the owner."
	label := "borrow origin"
	if record.LoopCarried {
		label = "borrow origin from a previous loop iteration"
	} else if isDeferredUseKind(record.Kind) {
		label = "defer retains this storage"
		help = borrowOverlapExplanation(place, record) + " The registered defer still needs this storage when cleanup runs. Keep the required value or reference holder live until then, or restructure the defer and ownership operation."
	} else if index := callReservationArgumentIndex(record); index > 0 {
		label = fmt.Sprintf("borrow prepared for argument %d", index)
	}
	endLine, endColumn := token.EndPosition()
	diagnostic := Error{
		Message: fmt.Sprintf(format, args...), Help: help,
		File: token.File, Line: token.Line, Column: token.Column,
		EndLine: endLine, EndColumn: endColumn,
		PreviousFile: record.Token.File, PreviousLine: record.Token.Line, PreviousColumn: record.Token.Column,
		RelatedLabel: label,
	}
	if holder, ok := a.symbols[record.Holder]; ok {
		holderEndLine, holderEndColumn := holder.Token.EndPosition()
		diagnostic.Related = append(diagnostic.Related, RelatedLocation{
			File: holder.Token.File, Line: holder.Token.Line, Column: holder.Token.Column,
			EndLine: holderEndLine, EndColumn: holderEndColumn,
			Label: "borrow holder " + record.Holder + " keeps authority live",
		})
	}
	a.appendError(diagnostic)
}
