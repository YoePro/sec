package main

import (
	"sec/internal/lexer"
	defaultactions "sec/internal/lsp/features/defaults"
	"sec/internal/parser"
)

// defaultCodeActions makes Sema-proved implicit defaults explicit only on an
// opt-in refactoring request. Every exact replacement is reparsed and analyzed
// with the same target and open-document overlays before publication.
// Rules: rules/types/default_values.md — "LSP", "Formatter";
// rules/tooling/lsp.md — "Snapshots", "Safe fixes", "Target-aware analysis".
func defaultCodeActions(uri, text string, requested lspRange, overlay sourceOverlay) []codeAction {
	path := pathFromURI(uri)
	parsed := parser.New(lexer.NewWithFile(text, path)).Parse()
	if parsed.Fatal || len(parsed.Diagnostics) != 0 {
		return nil
	}
	sites := defaultactions.Capture(text, parsed.Program)
	prepareProgramForLSP(parsed.Program, path, overlay)
	analyzer := newLSPAnalyzerWithOverlay(uri, parsed.Program, overlay)
	if len(analyzer.Analyze(parsed.Program)) != 0 {
		return nil
	}
	var result []codeAction
	for _, action := range defaultactions.Actions(text, analyzer, sites) {
		span := lspRange{Start: offsetPosition(text, action.Site.Start), End: offsetPosition(text, action.Site.End)}
		if comparePosition(span.End, requested.Start) < 0 || comparePosition(span.Start, requested.End) > 0 {
			continue
		}
		if action.Start < 0 || action.End < action.Start || action.End > len(text) {
			continue
		}
		edited := text[:action.Start] + action.Text + text[action.End:]
		checked := parser.New(lexer.NewWithFile(edited, path)).Parse()
		if checked.Fatal || len(checked.Diagnostics) != 0 {
			continue
		}
		updated := sourceOverlay{Sources: map[string]string{}, Targets: overlay.Targets}
		for file, source := range overlay.Sources {
			updated.Sources[file] = source
		}
		updated.Sources[normalizedSourcePath(path)] = edited
		prepareProgramForLSP(checked.Program, path, updated)
		if len(newLSPAnalyzerWithOverlay(uri, checked.Program, updated).Analyze(checked.Program)) != 0 {
			continue
		}
		result = append(result, codeAction{Title: action.Title, Kind: "refactor.rewrite", Edit: workspaceEdit{Changes: map[string][]textEdit{
			uri: {{Range: lspRange{Start: offsetPosition(text, action.Start), End: offsetPosition(text, action.End)}, NewText: action.Text}},
		}}})
	}
	return result
}
