package main

import (
	"os"

	"sec/internal/formatter"
)

// formatDocument formats the current snapshot (or unopened file) using the
// shared project configuration and returns one enclosing edit with unchanged
// source excluded. The protocol adapter uses UTF-16 positions; the shared
// formatter owns both canonical bytes and safe byte boundaries.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(22), § 28(1)–(2), § 29
//   - rules/tooling/lsp.md — "Shared formatter", "Formatting"
func (s *server) formatDocument(uri string) ([]textEdit, error) {
	snapshot, ok := s.documentSnapshots.Snapshot(uri)
	text := snapshot.Text
	if !ok {
		data, err := os.ReadFile(pathFromURI(uri))
		if err != nil {
			return nil, err
		}
		text = string(data)
	}

	formatted, err := formatSourceForDocument(uri, text)
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
