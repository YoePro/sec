package sema

import (
	"strconv"

	"sec/internal/lexer"
)

// DiagnosticCauseNote is one ordered explanation supplied by an analysis owner.
// Arguments retain domain-specific facts; Source may be absent and must never
// be reconstructed from message text. An incomplete chain is an explicit note.
type DiagnosticCauseNote struct {
	Key, Message string
	Source       lexer.Token
	Arguments    map[string]string
}

// DiagnosticProvenance is a detached common presentation of the locations and
// cause chains already attached to an occurrence by its semantic producers.
type DiagnosticProvenance struct {
	Related []RelatedLocation
	Notes   []DiagnosticCauseNote
}

// DiagnosticProvenance standardizes transport across analysis domains without
// replacing their proof lattices or creating a second causal analysis. Ordinary
// ownership/lifetime/bounds/stack provenance uses Related; escape and allocation
// witnesses additionally publish ordered notes, including incomplete coverage.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), §58(2–5), §59(1–3);
// rules/analysis/escape_analysis.md — Cause paths, Diagnostic quality;
// rules/memory/allocation.md — §§28(4),29(4);
// rules/tooling/diagnostics.md — §§8–10,14.
func (e Error) DiagnosticProvenance() DiagnosticProvenance {
	result := DiagnosticProvenance{Related: e.RelatedLocations()}
	add := func(key, message string, source lexer.Token, arguments map[string]string) {
		args := make(map[string]string, len(arguments)+3)
		for key, value := range arguments {
			args[key] = value
		}
		if source.File != "" && source.Line > 0 && source.Column > 0 {
			args["file"], args["line"], args["column"] = source.File, strconv.Itoa(source.Line), strconv.Itoa(source.Column)
		}
		result.Notes = append(result.Notes, DiagnosticCauseNote{Key: key, Message: message, Source: source, Arguments: args})
	}
	for index, path := range e.EscapeCauses {
		args := map[string]string{"path": strconv.Itoa(index), "mode": string(path.Mode), "destination": string(path.Destination), "incomplete": strconv.FormatBool(path.Incomplete)}
		for _, step := range path.Steps {
			args["step"] = step.Kind
			add("escape.cause."+step.Kind, step.Message, step.Source, args)
		}
		if path.Incomplete {
			delete(args, "step")
			add("escape.cause.incomplete", "escape cause path is incomplete", lexer.Token{}, args)
		}
	}
	if path := e.AllocationCause; path != nil {
		for _, step := range path.Steps {
			args := map[string]string{"unknown": strconv.FormatBool(path.Unknown), "caller": string(step.Caller), "callee": string(step.Callee), "site": string(step.Site), "incomplete": strconv.FormatBool(path.Incomplete), "step": step.Kind}
			add("allocation.cause."+step.Kind, step.Message, step.Source, args)
		}
		if path.Incomplete {
			add("allocation.cause.incomplete", "allocation cause path is incomplete", lexer.Token{}, map[string]string{"unknown": strconv.FormatBool(path.Unknown), "incomplete": "true"})
		}
	}
	return result
}
