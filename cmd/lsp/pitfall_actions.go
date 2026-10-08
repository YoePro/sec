package main

import (
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/fixes"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// pitfallCodeActions recomputes compiler findings from the current snapshot,
// depth and policy instead of trusting diagnostics supplied by the client.
// Certified edits share the compiler fix materializer; suggestions remain
// explicitly disabled manual guidance and never become an automatic repair.
// Rules: rules/analysis/pitfall_analysis.md — "LSP configuration reload",
// "Corrective actions", "Fix safety", "Diagnostic ownership and coalescing";
// rules/tooling/lsp.md — "Snapshots", "Safe fixes".
func pitfallCodeActions(uri, text string, requested lspRange, overlay sourceOverlay, settings pitfallDiagnosticSettings) []codeAction {
	result := []codeAction{}
	path := pathFromURI(uri)
	parsed := parser.New(lexer.NewWithFile(text, path)).Parse()
	if parsed.Fatal || len(parsed.Diagnostics) != 0 {
		return result
	}
	prepareProgramForLSP(parsed.Program, path, overlay)
	analyzer := newLSPAnalyzerWithOverlay(uri, parsed.Program, overlay)
	analyzer.Analyze(parsed.Program)
	severity := pitfallLevel(uri, settings)
	for _, finding := range analyzer.PitfallAnalysis().Findings() {
		if normalizedSourcePath(finding.Subject.Source.File) != normalizedSourcePath(path) {
			continue
		}
		// Mandatory owners survive optional off; optional actions follow their
		// corresponding diagnostic policy.
		if finding.DiagnosticID == "" && (severity == 0 || finding.Classification == sema.PitfallProvenInvalid) {
			continue
		}
		span := diagnosticTokenRange(finding.Subject.Source, text)
		if comparePosition(span.End, requested.Start) < 0 || comparePosition(span.Start, requested.End) > 0 {
			continue
		}
		code := finding.DiagnosticID
		level := 1
		if code == "" {
			code = diagnostics.PitfallAdvisory
			level = severity
		}
		d := diagnostic{Range: span, Code: code, Severity: level, Source: "sec", Message: string(finding.Rule)}
		for _, suggestion := range finding.Actions {
			action := codeAction{Title: "Suggested edit: " + suggestion.Title, Kind: "quickfix", Diagnostics: []diagnostic{d}}
			if edit, ok := fixes.Pitfall(text, path, finding, suggestion); ok {
				action.Title = "Proven fix: " + suggestion.Title
				action.Edit = workspaceEdit{Changes: map[string][]textEdit{uri: {{Range: lspRange{Start: offsetPosition(text, edit.Start), End: offsetPosition(text, edit.End)}, NewText: edit.Replacement}}}}
			} else {
				reason := "This suggestion requires a decision about intent; no automatic edit is certified."
				if suggestion.Kind == sema.PitfallProvenFix {
					reason = "The compiler proved equivalence, but an exact source replacement could not be verified."
				}
				if suggestion.Replacement != "" {
					reason += " Suggested replacement: " + strings.TrimSpace(suggestion.Replacement)
				}
				action.Disabled = &codeActionDisabled{Reason: reason}
			}
			result = append(result, action)
		}
	}
	return result
}
