package main

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/ir/semantic"
	"sec/internal/sema"
)

// semanticDiagnosticFailure is a boundary adapter for owner-produced proof
// failures. It exposes structured occurrences without parsing error strings.
// Rules: rules/compiler/compiler_analysis.md — §§6(4), 7(3–8), 58(3).
type semanticDiagnosticFailure interface {
	SemanticDiagnostics() []sema.Error
}

// reportPipelineError preserves owner-supplied proof states and locations
// through output commands. Wrappers and joined failures retain every cause;
// unsupported representation remains an implementation capability failure,
// and unrelated tool failures receive no fabricated source proof state.
// Rules: rules/compiler/compiler_pipeline.md — §§2(8–9), 32(2–4), 33(3);
// rules/compiler/compiler_analysis.md — §§7(4–8), 58(3);
// rules/tooling/diagnostics.md — §§8–10, 14, 28.
func reportPipelineError(stage string, err error) {
	cliDiagnostics.pipelineError(stage, err)
}

// pipelineError emits separate occurrences for independent joined failures,
// unwrapping error chains only to find structured owner facts. The outer
// wrapper explanation is retained as a note on the underlying occurrence.
// Rules: rules/compiler/compiler_analysis.md — §7(4–8), §58(3);
// rules/tooling/diagnostics.md — §§8–10, 14.
func (r *diagnosticReporter) pipelineError(stage string, err error) {
	if err == nil {
		return
	}
	switch failure := err.(type) {
	case semanticDiagnosticFailure:
		values := failure.SemanticDiagnostics()
		if len(values) != 0 {
			for _, value := range values {
				r.pipelineSemanticDiagnostic(stage, value)
			}
			return
		}
	case sema.Error:
		r.pipelineSemanticDiagnostic(stage, failure)
		return
	case *sema.Error:
		r.pipelineError(stage, *failure)
		return
	case *semantic.UnsupportedFeatureError:
		occurrence := newOccurrence("", diagnostics.SeverityError, stage, failure.Error())
		occurrence.Arguments["category"] = "unsupported-lowering"
		occurrence.Message.Arguments["category"] = "unsupported-lowering"
		occurrence.addHelp("The selected compiler path cannot lower this construct yet. This is an implementation capability limitation, not a proven source violation or a missing language-validity proof.")
		if failure.Location.Line > 0 && failure.Location.Column > 0 {
			position := occurrencePosition{Line: failure.Location.Line, Column: failure.Location.Column}
			occurrence.Primary = &occurrenceLocation{Span: occurrenceSpan{File: failure.Location.File, Start: position, End: position}}
		}
		r.record(occurrence, fmt.Sprintf("%s error [unsupported-lowering]: %s\n  help: %s\n", stage, failure, occurrence.Help[0].Text))
		return
	case interface{ Unwrap() []error }:
		causes := failure.Unwrap()
		if len(causes) != 0 {
			for _, cause := range causes {
				r.pipelineError(stage, cause)
			}
			return
		}
	case interface{ Unwrap() error }:
		if cause := failure.Unwrap(); cause != nil {
			before := len(r.occurrences)
			r.pipelineError(stage, cause)
			for index := before; index < len(r.occurrences); index++ {
				r.occurrences[index].Notes = append(r.occurrences[index].Notes,
					occurrenceMessage{Key: "pipeline.context", Arguments: map[string]string{}, Text: err.Error()})
			}
			if r.format == diagnosticFormatHuman {
				fmt.Fprintf(r.output, "  context: %s\n", err)
			}
			return
		}
	}
	occurrence := newOccurrence("", diagnostics.SeverityError, stage, err.Error())
	r.record(occurrence, fmt.Sprintf("%s error: %s\n", stage, err))
}

// pipelineSemanticDiagnostic renders the same mandatory/advisory occurrence
// at a pipeline boundary with its original identity, help and proof outcome.
// Rules: rules/compiler/compiler_analysis.md — §58(3);
// rules/tooling/diagnostics.md — §§5, 10, 14.
func (r *diagnosticReporter) pipelineSemanticDiagnostic(stage string, value sema.Error) {
	identity := ""
	if value.ID != "" {
		identity = "[" + value.ID + "]"
	}
	severity := value.Severity
	if severity == "" {
		severity = diagnostics.SeverityError
	}
	human := fmt.Sprintf("%s %s%s: %s\n", stage, severity, identity, value)
	if value.Help != "" {
		human += "  help: " + value.Help + "\n"
	}
	r.semaDiagnostic(value, human, stage)
}
