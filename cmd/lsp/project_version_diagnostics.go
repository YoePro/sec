package main

import (
	"path/filepath"

	"sec/internal/sema"
)

// isProjectManifest distinguishes TOML documents from Sec sources.
// Rules: rules/projects/projects.md — §5(1–2).
func isProjectManifest(path string) bool {
	return filepath.Base(path) == "sec.toml" && filepath.Base(filepath.Dir(path)) == ".sec"
}

// projectVersionDiagnostics renders shared compiler occurrences on the actual
// manifest snapshot using the negotiated protocol position conversion.
// Rules: rules/projects/projects.md — §48(1–3); rules/tooling/lsp.md — shared diagnostics.
func projectVersionDiagnostics(uri, text string) []diagnostic {
	result := []diagnostic{}
	for _, issue := range sema.ValidateProjectVersion(pathFromURI(uri), text) {
		result = append(result, semaDiagnosticWithSources(issue, 1, uri, text, sourceOverlay{}))
	}
	return result
}

// collectProjectVersionDiagnostics includes governing manifests even when only
// child source files are open. Empty results allow stale errors to be cleared.
// Rules: rules/projects/projects.md — §§36,48; rules/tooling/lsp.md — snapshots.
func collectProjectVersionDiagnostics(source string, overlay sourceOverlay, results map[string][]diagnostic) {
	if path, text, found := lspProjectManifest(source, overlay); found {
		uri := uriFromPath(path)
		results[uri] = projectVersionDiagnostics(uri, text)
	}
}
