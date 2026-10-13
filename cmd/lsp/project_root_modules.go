package main

import (
	"path/filepath"

	"sec/internal/ast"
	lspserver "sec/internal/lsp/server"
	"sec/internal/sema"
)

// lspProjectRootModuleErrors uses the same compiler validation and resolved
// root naming and source-directory contracts as CLI; an unsaved manifest wins over its disk copy.
// Rules: rules/projects/projects.md — §§14–15; rules/tooling/lsp.md — shared compiler workspace.
func lspProjectRootModuleErrors(program *ast.Program, sourceFile string, overlay sourceOverlay) []sema.Error {
	path, text, found := lspProjectManifest(sourceFile, overlay)
	if !found {
		return nil
	}
	return append(sema.ValidateProjectRootModules(program, path, text), sema.ValidateProjectSourceLayout(program, path)...)
}

// lspProjectManifest uses the nearest disk or unsaved-overlay manifest.
// Rules: rules/projects/projects.md — §36; rules/tooling/lsp.md — snapshots.
func lspProjectManifest(sourceFile string, overlay sourceOverlay) (string, string, bool) {
	for dir := filepath.Dir(normalizedSourcePath(sourceFile)); ; dir = filepath.Dir(dir) {
		path := filepath.Join(dir, ".sec", "sec.toml")
		if text, err := lspserver.ReadSource(path, overlay.Sources); err == nil {
			return path, string(text), true
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return "", "", false
}

// lspProjectSourceLayoutErrors checks imported sources before recursive resolution.
// Rules: rules/projects/projects.md — §15; rules/projects/modules.md — §13.
func lspProjectSourceLayoutErrors(program *ast.Program, sourceFile string, overlay sourceOverlay) []sema.Error {
	path, _, found := lspProjectManifest(sourceFile, overlay)
	if !found {
		return nil
	}
	return sema.ValidateProjectSourceLayout(program, path)
}
