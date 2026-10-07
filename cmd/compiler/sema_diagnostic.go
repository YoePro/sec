package main

import (
	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// semaDiagnostic records one semantic diagnostic, including its related
// earlier location, help and explicit proof state. An optional pipeline stage
// records the consuming boundary without replacing the semantic producer.
// Rules: rules/tooling/diagnostics.md — §§8–10, 14;
// rules/compiler/compiler_analysis.md — §7(4–8), §58(3); rules/memory/allocation.md — §29(4).
func (r *diagnosticReporter) semaDiagnostic(diagnostic sema.Error, human string, stage ...string) {
	severity := diagnostic.Severity
	if severity == "" {
		severity = diagnostics.SeverityError
	}
	occurrence := newOccurrence(diagnostic.ID, severity, "sema", diagnostic.DisplayMessage())
	if len(stage) > 0 {
		occurrence.Arguments["pipeline_stage"] = stage[0]
		occurrence.Message.Arguments["pipeline_stage"] = stage[0]
	}
	if diagnostic.ProofState != "" {
		occurrence.Arguments["proof_state"] = string(diagnostic.ProofState)
		occurrence.Message.Arguments["proof_state"] = string(diagnostic.ProofState)
	}
	if diagnostic.Line > 0 && diagnostic.Column > 0 {
		endLine, endColumn := diagnostic.EndLine, diagnostic.EndColumn
		if endLine == 0 {
			endLine, endColumn = diagnostic.Line, diagnostic.Column
		}
		occurrence.Primary = &occurrenceLocation{Span: occurrenceSpan{
			File:  diagnostic.File,
			Start: occurrencePosition{Line: diagnostic.Line, Column: diagnostic.Column},
			End:   occurrencePosition{Line: endLine, Column: endColumn},
		}}
	}
	if diagnostic.PreviousLine > 0 && diagnostic.PreviousColumn > 0 {
		position := occurrencePosition{Line: diagnostic.PreviousLine, Column: diagnostic.PreviousColumn}
		related := occurrenceRelated{
			Span: occurrenceSpan{File: diagnostic.PreviousFile, Start: position, End: position},
		}
		if diagnostic.RelatedLabel != "" {
			related.Message = &occurrenceMessage{Key: "related", Arguments: map[string]string{}, Text: diagnostic.RelatedLabel}
		}
		occurrence.Related = append(occurrence.Related, related)
	}
	occurrence.addHelp(diagnostic.Help)
	human = appendEscapeCausePaths(&occurrence, diagnostic, human)
	human = appendAllocationCausePath(&occurrence, diagnostic, human)
	r.record(occurrence, human)
}
