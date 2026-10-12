package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// addUnavailableOwnershipDiagnostic links rejected use to the actual ownership
// operation and original owner. It retains path uncertainty and the existing
// mandatory ID/help, without inventing missing origins or automatic edits.
// Rules: rules/memory/copy_move.md — §§28.1(1–3),29(1);
// rules/memory/destruction.md — §33.1(1–2),§33.2(1);
// rules/memory/ownership.md — §§6,33.2–33.3;
// rules/corrections/applied/diagnostics-ownership-v2-correction-20260826.md — "Required explanation shape".
func (a *Analyzer) addUnavailableOwnershipDiagnostic(place Place, movedKey string, token, movedAt lexer.Token, id, help, format string, args ...any) {
	reason := underlyingAvailabilityReason(a.moveReasons[movedKey])
	label := "ownership " + availabilityReasonPhrase(reason) + " here"
	explanation := fmt.Sprintf("%s no longer owns a usable value after it was %s at the related ownership action.", place.String(), availabilityReasonPhrase(reason))
	if id == diagnostics.ConditionallyUnavailableUse {
		label = "ownership action on a possibly unavailable path"
		explanation = fmt.Sprintf("%s may lack its owned value on a continuing path after %s was %s. The related action is one path witness, not proof that every path consumed it.", place.String(), movedKey, availabilityReasonPhrase(reason))
	} else if id == diagnostics.PartiallyUnavailableUse {
		explanation = fmt.Sprintf("%s cannot be used as a whole because sub-Place %s was %s and no longer contains its owned value.", place.String(), movedKey, availabilityReasonPhrase(reason))
	} else if isMultipleAvailabilityReason(reason) {
		label = "earlier ownership action on one continuing path"
		explanation = fmt.Sprintf("Every continuing path leaves %s unavailable after it was %s. The related action is one witness; the paths may end ownership in different ways.", place.String(), availabilityReasonPhrase(reason))
	}
	endLine, endColumn := token.EndPosition()
	a.appendError(Error{
		ID: id, Message: fmt.Sprintf(format, args...), Help: explanation + " " + help,
		File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn,
		PreviousFile: movedAt.File, PreviousLine: movedAt.Line, PreviousColumn: movedAt.Column, RelatedLabel: label,
		Related: []RelatedLocation{ownershipSourceLocation(place)},
	})
}
