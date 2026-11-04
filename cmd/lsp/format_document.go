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
	snapshot, ok := s.documentSnapshots.Snapshot(uri)
	text := snapshot.Text
	if !ok {
		data, err := os.ReadFile(pathFromURI(uri))
		if err != nil {
			return nil, err
		}
		text = string(data)
	}

	formatted, err := formatSourceForDocument(uri, text, presentationWidths...)
	if err != nil {
		return nil, err
	}
	edit, changed := formatter.MinimalEdit(text, formatted)
	if !changed {
		return []textEdit{}, nil
	}
	return []textEdit{{
		Range: lspRange{
			Start: offsetPosition(text, edit.Start),
			End:   offsetPosition(text, edit.End),
		},
		NewText: edit.NewText,
	}}, nil
}
