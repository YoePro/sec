package main

import "sec/internal/sema"

// appendDiagnosticCauseNotes projects the same owner notes as CLI, converting
// each navigable point with its own document/overlay UTF-16 source coordinates.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), §58(2–5);
// rules/tooling/lsp.md — Shared diagnostic model, protocol position encoding.
func appendDiagnosticCauseNotes(err sema.Error, notes []sema.DiagnosticCauseNote, message string, related []diagnosticRelatedInformation, uri, text string, overlay sourceOverlay) (string, []diagnosticRelatedInformation) {
	for _, note := range notes {
		message += "\n\nnote: " + note.Message
		token := note.Source
		if token.File == "" || token.Line <= 0 || token.Column <= 0 {
			continue
		}
		if token.File == err.File && token.Line == err.Line && token.Column == err.Column {
			continue
		}
		point := position{Line: token.Line - 1, Character: token.Column - 1}
		if source := sourceTextForToken(uri, text, overlay, token); source != "" {
			point = diagnosticTokenStart(source, token)
		}
		target := uriFromPath(token.File)
		known := false
		for _, previous := range related {
			known = known || previous.Location.URI == target && previous.Location.Range.Start == point
		}
		if !known {
			related = append(related, diagnosticRelatedInformation{Location: location{URI: target, Range: lspRange{Start: point, End: point}}, Message: note.Message})
		}
	}
	return message, related
}
