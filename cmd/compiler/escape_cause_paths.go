package main

import (
	"fmt"
	"sec/internal/sema"
	"strconv"
)

// appendEscapeCausePaths transports canonical explanations as ordered JSON
// notes and source links, retaining one root occurrence and its stable ID.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic quality";
// rules/tooling/diagnostics.md — §§8–10, 14.
func appendEscapeCausePaths(occurrence *emittedOccurrence, diagnostic sema.Error, human string) string {
	for index, path := range diagnostic.EscapeCauses {
		for _, step := range path.Steps {
			arguments := map[string]string{"path": strconv.Itoa(index), "mode": string(path.Mode), "destination": string(path.Destination), "incomplete": strconv.FormatBool(path.Incomplete), "step": step.Kind}
			message := step.Message
			if step.Source.Line > 0 && step.Source.Column > 0 {
				arguments["file"] = step.Source.File
				arguments["line"] = strconv.Itoa(step.Source.Line)
				arguments["column"] = strconv.Itoa(step.Source.Column)
				message += fmt.Sprintf(" at %s:%d:%d", step.Source.File, step.Source.Line, step.Source.Column)
				position := occurrencePosition{Line: step.Source.Line, Column: step.Source.Column}
				span := occurrenceSpan{File: step.Source.File, Start: position, End: position}
				known := occurrence.Primary != nil && occurrence.Primary.Span.File == span.File && occurrence.Primary.Span.Start == position
				for _, related := range occurrence.Related {
					known = known || related.Span.File == span.File && related.Span.Start == position
				}
				if !known {
					occurrence.Related = append(occurrence.Related, occurrenceRelated{Span: span, Message: &occurrenceMessage{Key: "escape.cause." + step.Kind, Arguments: arguments, Text: step.Message}})
				}
			}
			occurrence.Notes = append(occurrence.Notes, occurrenceMessage{Key: "escape.cause." + step.Kind, Arguments: arguments, Text: message})
			human += "note: " + message + "\n"
		}
		if path.Incomplete {
			human += "note: escape cause path is incomplete\n"
		}
	}
	return human
}
