package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// addBorrowAuthorityConflict explains ownership presence independently from
// borrow authority, using the current compiler-owned availability state. It
// retains the owning message and both conflict/origin source locations, and
// explains overlap using the canonical relationship. The owning semantic check
// supplies its stable category; loop-carried state has a dedicated identity.
// Rules: rules/memory/borrowing.md — §§6(1–7),32(2–4,6–8),33(1,3);
// rules/tooling/diagnostics.md — §§5–7,9,15,17.
func (a *Analyzer) addBorrowAuthorityConflict(place Place, token lexer.Token, record borrowRecord, id string, format string, args ...any) {
	if record.LoopCarried {
		id = diagnostics.BorrowLoopCarriedConflict
	}
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
		ID: id, Severity: diagnostics.SeverityError,
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
