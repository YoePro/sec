package main

import (
	"sec/internal/sema"
)

// appendAllocationCausePath renders the compiler's source-mapped witness and
// navigable intermediate locations using each file's UTF-16 coordinates.
// Rules: rules/memory/allocation.md — §§28(4),29(4);
// rules/tooling/lsp.md — "Shared diagnostic model", protocol position encoding.
func appendAllocationCausePath(err sema.Error, message string, related []diagnosticRelatedInformation, uri, text string, overlay sourceOverlay) (string, []diagnosticRelatedInformation) {
	if err.AllocationCause != nil {
		path := err.AllocationCause
		for _, step := range path.Steps {
			message += "\n\nnote: " + step.Message
			token := step.Source
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
				related = append(related, diagnosticRelatedInformation{Location: location{URI: target, Range: lspRange{Start: point, End: point}}, Message: step.Message})
			}
		}
		if path.Incomplete {
			message += "\n\nnote: allocation cause path is incomplete"
		}
	}
	return message, related
}
