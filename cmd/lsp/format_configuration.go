package main

import (
	"sec/internal/formatconfig"
	"sec/internal/formatter"
)

// formatSourceForDocument formats one document with the same effective
// configuration as `sec fmt`: the `[format]` table of the document's project
// `.sec/sec.toml`, or the formatter defaults for a stand-alone file. Editor
// preferences never override the project configuration, and an invalid
// configuration returns an error and no edit.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(15), § 4(18)–(23)
//   - rules/tooling/lsp.md — "Shared formatter"
func formatSourceForDocument(uri string, text string, presentationWidths ...uint64) (string, error) {
	settings, err := formatconfig.ForSource(pathFromURI(uri), formatconfig.Overrides{})
	if err != nil {
		return "", err
	}
	var width uint64
	if len(presentationWidths) > 0 {
		width = presentationWidths[0]
	}
	return formatter.Format(formatter.Source{Text: text}, settings.EffectiveOptions(width)).Text, nil
}
