package main

import (
	"fmt"
	"sec/internal/sema"
)

// appendDiagnosticCauseNotes transports the common Sema provenance view as
// ordered human/JSON notes and navigable source links. Missing source points
// remain text-only; primary and related locations are never duplicated.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), §58(2–5);
// rules/tooling/diagnostics.md — §§8–10,14.
func appendDiagnosticCauseNotes(occurrence *emittedOccurrence, notes []sema.DiagnosticCauseNote, human string) string {
	for _, note := range notes {
		message := note.Message
		token := note.Source
		if token.File != "" && token.Line > 0 && token.Column > 0 {
			message += fmt.Sprintf(" at %s:%d:%d", token.File, token.Line, token.Column)
			point := occurrencePosition{Line: token.Line, Column: token.Column}
			span := occurrenceSpan{File: token.File, Start: point, End: point}
			known := occurrence.Primary != nil && occurrence.Primary.Span.File == span.File && occurrence.Primary.Span.Start == point
			for _, related := range occurrence.Related {
				known = known || related.Span.File == span.File && related.Span.Start == point
			}
			if !known {
				occurrence.Related = append(occurrence.Related, occurrenceRelated{Span: span, Message: &occurrenceMessage{Key: note.Key, Arguments: note.Arguments, Text: note.Message}})
			}
		}
		occurrence.Notes = append(occurrence.Notes, occurrenceMessage{Key: note.Key, Arguments: note.Arguments, Text: message})
		human += "note: " + message + "\n"
	}
	return human
}
