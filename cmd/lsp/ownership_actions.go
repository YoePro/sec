package main

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// ownershipCodeActions offers an explicit-move repair only for a current
// compiler-owned implicit-copy diagnostic and a replacement accepted by Sema.
// All assembled source errors block the repair; fewer errors is not enough
// to prove that a new later use, borrow or cleanup violation was not introduced.
// Rules: rules/memory/copy_move.md — §29(1),(4);
// rules/tooling/lsp.md — "Safe fixes", "Document snapshots", "Target-aware analysis".
func ownershipCodeActions(uri string, text string, reported []diagnostic, overlays ...sourceOverlay) (actions []codeAction) {
	actions = []codeAction{}
	candidates := []diagnostic{}
	for _, diagnostic := range reported {
		if diagnostic.Code == diagnostics.ImplicitMoveDisallowed {
			candidates = append(candidates, diagnostic)
		}
	}
	if len(candidates) == 0 {
		return actions
	}
	overlay := firstSourceOverlay(overlays)
	baseline, complete := ownershipActionErrors(uri, text, overlay)
	if !complete {
		return actions
	}
	seen := map[lspRange]bool{}
	for _, reportedDiagnostic := range candidates {
		current := false
		for _, error := range baseline {
			if error.ID != diagnostics.ImplicitMoveDisallowed || !diagnosticBelongsToSource(error, pathFromURI(uri)) {
				continue
			}
			canonical := semaDiagnosticWithSources(error, 1, uri, text, overlay)
			if canonical.Range == reportedDiagnostic.Range {
				current = true
				break
			}
		}
		if !current {
			continue
		}
		edit, title, ok := ownershipMoveSyntaxEdit(text, reportedDiagnostic)
		if !ok || seen[edit.Range] {
			continue
		}
		// Duplicate client diagnostics must not repeat the same Sema query,
		// including candidates whose consumption fails validation.
		seen[edit.Range] = true
		edited := applyTextEdits(text, []textEdit{edit})
		errors, complete := ownershipActionErrors(uri, edited, overlay)
		if !complete || len(errors) != 0 {
			continue
		}
		actions = append(actions, codeAction{Title: title, Kind: "quickfix", Diagnostics: []diagnostic{reportedDiagnostic},
			Edit: workspaceEdit{Changes: map[string][]textEdit{uri: {edit}}}})
	}
	return actions
}

// ownershipActionErrors reuses the compiler parser, module/import assembly and
// selected-target Sema with an authoritative replacement of the active overlay.
// Parse/import failure or a compiler panic cannot establish safe consumption.
// Errors from other assembled source files are retained rather than hidden by
// the document-only diagnostic projection used by ordinary LSP publication.
// Rules: rules/memory/copy_move.md — §29(4);
// rules/tooling/lsp.md — "Safe fixes", "Document snapshots", "Unknown facts".
func ownershipActionErrors(uri, text string, overlay sourceOverlay) (errors []sema.Error, complete bool) {
	defer func() {
		if recover() != nil {
			errors = nil
			complete = false
		}
	}()
	path := pathFromURI(uri)
	updated := sourceOverlay{Sources: map[string]string{}, Targets: overlay.Targets}
	for file, source := range overlay.Sources {
		updated.Sources[file] = source
	}
	if path != "" {
		updated.Sources[normalizedSourcePath(path)] = text
	}
	parsed := parser.New(lexer.NewWithFile(text, path)).Parse()
	if parsed.Fatal || len(parsed.Diagnostics) != 0 {
		return nil, false
	}
	if errors := prepareProgramForLSP(parsed.Program, path, updated); len(errors) != 0 {
		return errors, false
	}
	analyzer := newLSPAnalyzerWithOverlay(uri, parsed.Program, updated)
	return analyzer.Analyze(parsed.Program), true
}

// ownershipMoveSyntaxEdit turns only the operator belonging to a verified
// implicit-move diagnostic into its explicit-move equivalent. The diagnostic
// is attached to the named source, so the immediately preceding token must be
// the ordinary declaration or assignment operator. Semantic admission belongs
// to ownershipCodeActions; this helper alone establishes no ownership proof.
// Rules: rules/memory/copy_move.md — §29(3–4).
func ownershipMoveSyntaxEdit(text string, reported diagnostic) (textEdit, string, bool) {
	targetOffset := lineCharToOffset(text, reported.Range.Start.Line, reported.Range.Start.Character)
	l := lexer.New(text)
	previous := lexer.Token{}
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			return textEdit{}, "", false
		}
		if textPositionOffset(text, token.Line, token.Column) == targetOffset {
			var replacement, title string
			switch previous.Type {
			case lexer.DECLARE:
				replacement = ":<-"
				title = "Change := to :<- to move the value (source becomes unavailable)"
			case lexer.ASSIGN:
				replacement = "<-"
				title = "Change = to <- to move the value (source becomes unavailable)"
			default:
				return textEdit{}, "", false
			}
			return textEdit{
				Range:   tokenRange(text, previous),
				NewText: replacement,
			}, title, true
		}
		previous = token
	}
}
