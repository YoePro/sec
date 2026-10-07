package main

import (
	"fmt"
	"sec/internal/sema"
	"strconv"
)

// appendAllocationCausePath transports canonical allocation explanations as ordered JSON
// notes and source links, retaining one root occurrence and its stable ID.
// Rules: rules/memory/allocation.md — §§28(4),29(4);
// rules/tooling/diagnostics.md — §§8–10, 14.
func appendAllocationCausePath(occurrence *emittedOccurrence, diagnostic sema.Error, human string) string {
	if diagnostic.AllocationCause != nil {
		path := diagnostic.AllocationCause
		for _, step := range path.Steps {
			arguments := map[string]string{"unknown": strconv.FormatBool(path.Unknown), "caller": string(step.Caller), "callee": string(step.Callee), "site": string(step.Site), "incomplete": strconv.FormatBool(path.Incomplete), "step": step.Kind}
			message := step.Message
			if step.Source.File != "" && step.Source.Line > 0 && step.Source.Column > 0 {
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
					occurrence.Related = append(occurrence.Related, occurrenceRelated{Span: span, Message: &occurrenceMessage{Key: "allocation.cause." + step.Kind, Arguments: arguments, Text: step.Message}})
				}
			}
			occurrence.Notes = append(occurrence.Notes, occurrenceMessage{Key: "allocation.cause." + step.Kind, Arguments: arguments, Text: message})
			human += "note: " + message + "\n"
		}
		if path.Incomplete {
			human += "note: allocation cause path is incomplete\n"
		}
	}
	return human
}
