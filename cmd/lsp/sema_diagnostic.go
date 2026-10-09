package main

import (
	"fmt"
	"sec/internal/diagnostics"
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
//   - rules/tooling/diagnostics.md — §4 severity and mandatory definitions

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
	for _, link := range err.RelatedLocations() {
		previous := fmt.Sprintf("%d:%d", link.Line, link.Column)
		if link.File != "" {
			previous = link.File + ":" + previous
		}
		message += "\n\n" + link.Label + " at " + previous
		if link.File != "" {
			token := lexer.Token{File: link.File, Line: link.Line, Column: link.Column}
			point := position{Line: link.Line - 1, Character: link.Column - 1}
			relatedText := sourceTextForToken(uri, text, overlay, token)
			if relatedText != "" {
				point = diagnosticTokenStart(relatedText, token)
			}
			end := point
			if link.EndLine > 0 && link.EndColumn > 0 {
				end = position{Line: link.EndLine - 1, Character: link.EndColumn - 1}
				if relatedText != "" {
					end = diagnosticTokenStart(relatedText, lexer.Token{Line: link.EndLine, Column: link.EndColumn})
				}
			}
			related = append(related, diagnosticRelatedInformation{Location: location{URI: uriFromPath(link.File), Range: lspRange{Start: point, End: end}}, Message: link.Label})
		}
	}

	if err.Help != "" {
		message += "\n\nhelp: " + err.Help
	}
	message, related = appendEscapeCausePaths(err, message, related, uri, text, overlay)
	message, related = appendAllocationCausePath(err, message, related, uri, text, overlay)
	resolvedSeverity := lspSeverity(err.Severity, severity)
	// Mandatory registry policy also applies when a lowering occurrence lacks
	// an explicit severity or arrives through an advisory fallback path.
	if definition, known := diagnostics.Lookup(err.ID); known && definition.Mandatory {
		resolvedSeverity = lspSeverity(definition.DefaultSeverity, resolvedSeverity)
	}
	return diagnostic{
		Range: lspRange{
			Start: start,
			End:   end,
		},
		Severity:           resolvedSeverity,
		Code:               err.ID,
		Source:             "sec",
		Message:            message,
		RelatedInformation: related,
	}
}
