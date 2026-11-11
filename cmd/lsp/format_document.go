package main

import (
	"os"

	"sec/internal/formatter"
)

type formattingParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Options      struct {
		// Optional observed viewport columns, not a personal formatting
		// preference. Fixed project widths ignore it; absent/zero uses the
		// project fallback. Rules: formatter.md §4(10–15,22–23).
		PresentationWidth uint64 `json:"secPresentationWidth,omitempty"`
	} `json:"options"`
}

// formatDocument formats the current snapshot (or unopened file) using the
// shared project configuration and returns one enclosing edit with unchanged
// source excluded. The protocol adapter uses UTF-16 positions; the shared
// formatter owns both canonical bytes and safe byte boundaries.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(22), § 28(1)–(2), § 29
//   - rules/tooling/lsp.md — "Shared formatter", "Formatting"
func (s *server) formatDocument(uri string, presentationWidths ...uint64) ([]textEdit, error) {
	text, err := s.formattingText(uri)
	if err != nil {
		return nil, err
	}
	formatted, err := formatSourceForDocument(uri, text, presentationWidths...)
	if err != nil {
		return nil, err
	}
	edit, changed := formatter.MinimalEdit(text, formatted)
	return formattingEdits(text, edit, changed), nil
}

// formattingText uses the latest synchronized document, falling back to disk.
// Rules: rules/tooling/lsp.md — snapshots, Appendix A.8.
func (s *server) formattingText(uri string) (string, error) {
	snapshot, ok := s.documentSnapshots.Snapshot(uri)
	text := snapshot.Text
	if !ok {
		data, err := os.ReadFile(pathFromURI(uri))
		if err != nil {
			return "", err
		}
		text = string(data)
	}
	return text, nil
}

// formattingEdits adapts safe shared byte edits to UTF-16 protocol positions.
// Rules: rules/tooling/formatter.md §28, §29; rules/tooling/lsp.md Appendix A.8.
func formattingEdits(text string, edit formatter.Edit, changed bool) []textEdit {
	if !changed {
		return []textEdit{}
	}
	return []textEdit{{
		Range: lspRange{
			Start: offsetPosition(text, edit.Start),
			End:   offsetPosition(text, edit.End),
		},
		NewText: edit.NewText,
	}}
}
