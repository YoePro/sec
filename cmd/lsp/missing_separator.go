package main

import (
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// missingSeparatorCodeActions offers the quick fix for a P2002 missing-comma
// diagnostic whose repair the parser proved: inserting ',' directly after the
// previous list item (struct field, parameter, argument, or array element),
// before any trailing comment. It is the same edit the opt-in formatter
// Language Correction applies.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Missing comma"
//   - rules/tooling/formatter.md — § 27(37), § 28(6)
//   - rules/tooling/lsp.md — "Code actions"
func missingSeparatorCodeActions(uri string, text string, reported []diagnostic) []codeAction {
	if len(reported) == 0 {
		return nil
	}
	result := parser.New(lexer.NewWithFile(text, pathFromURI(uri))).Parse()
	actions := []codeAction{}
	for _, event := range result.Recovery {
		if event.Kind != parser.RecoveryInsertMissingToken || event.After.Type == "" ||
			event.DiagnosticID != compilerdiagnostics.ParserMissingToken ||
			len(event.Expected) != 1 || event.Expected[0] != lexer.COMMA {
			continue
		}
		start := diagnosticTokenStart(text, event.Start)
		endLine, endColumn := event.After.EndPosition()
		insertAt := diagnosticTokenStart(text, lexer.Token{Line: endLine, Column: endColumn})
		for _, reportedDiagnostic := range reported {
			if reportedDiagnostic.Code != compilerdiagnostics.ParserMissingToken || reportedDiagnostic.Range.Start != start {
				continue
			}
			actions = append(actions, codeAction{
				Title:       "Insert missing ','",
				Kind:        "quickfix",
				Diagnostics: []diagnostic{reportedDiagnostic},
				Edit: workspaceEdit{Changes: map[string][]textEdit{
					uri: {{Range: lspRange{Start: insertAt, End: insertAt}, NewText: ","}},
				}},
			})
			break
		}
	}
	return actions
}
