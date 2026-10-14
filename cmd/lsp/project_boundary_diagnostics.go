package main

import (
	lspserver "sec/internal/lsp/server"
	"sec/internal/sema"
)

// analyzeProjectManifest validates the current manifest without parsing it as
// Sec source or mutating the caller's immutable overlay snapshot.
// Rules: rules/projects/projects.md — §48; rules/tooling/lsp.md — Snapshots.
func analyzeProjectManifest(uri, text string, overlay sourceOverlay) []diagnostic {
	path := normalizedSourcePath(pathFromURI(uri))
	sources := lspserver.SourceOverlay{}
	for path, text := range overlay.Sources {
		sources[path] = text
	}
	sources[path] = text
	overlay.Sources = sources
	canonicalURI := uriFromPath(path)
	results := map[string][]diagnostic{canonicalURI: projectVersionDiagnostics(canonicalURI, text)}
	collectProjectBoundaryDiagnostics(path, overlay, results, map[string]bool{})
	return results[canonicalURI]
}

// collectProjectBoundaryDiagnostics routes each shared boundary occurrence to
// its manifest snapshot. Discovery is coalesced within a batch, never cached
// across edits; unsaved manifests and ancestor manifests participate equally.
// Rules: rules/projects/projects.md — §§4,36,48; rules/tooling/lsp.md — Snapshots.
func collectProjectBoundaryDiagnostics(source string, overlay sourceOverlay, results map[string][]diagnostic, checked map[string]bool) {
	path, _, found := lspProjectManifest(source, overlay)
	if !found || checked[path] {
		return
	}
	checked[path] = true
	for _, issue := range sema.ValidateProjectBoundary(path, overlay.Sources) {
		checked[issue.File] = true
		if issue.PreviousFile != "" {
			checked[issue.PreviousFile] = true
		}
		uri := uriFromPath(issue.File)
		text, _ := lspserver.ReadSource(issue.File, overlay.Sources)
		results[uri] = append(results[uri], semaDiagnosticWithSources(issue, 1, uri, string(text), overlay))
	}
}
