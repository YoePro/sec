package main

import (
	"fmt"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// semaDiagnosticWithSources is semaDiagnostic with the current document URI
// and the open-document overlay, preserving explicit owner proof states so
// Unproven rejections cannot be presented as proven violations. A related
// location is converted to UTF-16
// with the related file's own text: the current document, an unsaved open
// document, or the file on disk. An unreadable related file keeps the scalar
// column rather than dropping the link.
//
// Rules:
//   - rules/tooling/lsp.md — "Shared diagnostic model", protocol position encoding
//   - rules/compiler/compiler_analysis.md — §7(4–8), §58(3)
//   - rules/memory/allocation.md — §29(4)

func semaDiagnosticWithSources(err sema.Error, severity int, uri string, text string, overlay sourceOverlay) diagnostic {
	start := diagnosticTokenStart(text, lexer.Token{Line: err.Line, Column: err.Column})
	end := start
	end.Character++
	if err.EndLine > 0 && err.EndColumn > 0 {
		end = diagnosticTokenStart(text, lexer.Token{Line: err.EndLine, Column: err.EndColumn})
	}
	// The editor already places the diagnostic at its range, so the message
	// carries only the related location rather than repeating the primary
	// one; the related location is also attached as a navigable link.
	message := err.DisplayMessage()
	var related []diagnosticRelatedInformation
	if err.PreviousLine > 0 && err.PreviousColumn > 0 {
		previous := fmt.Sprintf("%d:%d", err.PreviousLine, err.PreviousColumn)
		if err.PreviousFile != "" {
			previous = err.PreviousFile + ":" + previous
		}
		message += "\n\n" + err.RelatedLocationLabel() + " at " + previous
		if err.PreviousFile != "" {
			relatedToken := lexer.Token{File: err.PreviousFile, Line: err.PreviousLine, Column: err.PreviousColumn}
			point := position{Line: err.PreviousLine - 1, Character: err.PreviousColumn - 1}
			if relatedText := sourceTextForToken(uri, text, overlay, relatedToken); relatedText != "" {
				point = diagnosticTokenStart(relatedText, relatedToken)
			}
			related = append(related, diagnosticRelatedInformation{
				Location: location{URI: uriFromPath(err.PreviousFile), Range: lspRange{Start: point, End: point}},
				Message:  err.RelatedLocationLabel(),
			})
		}
	}
	if err.Help != "" {
		message += "\n\nhelp: " + err.Help
	}
	message, related = appendEscapeCausePaths(err, message, related, uri, text, overlay)
	message, related = appendAllocationCausePath(err, message, related, uri, text, overlay)
	return diagnostic{
		Range: lspRange{
			Start: start,
			End:   end,
		},
		Severity:           lspSeverity(err.Severity, severity),
		Code:               err.ID,
		Source:             "sec",
		Message:            message,
		RelatedInformation: related,
	}
}
