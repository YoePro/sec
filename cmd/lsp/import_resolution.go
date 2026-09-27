package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/modules"
	"sec/internal/sema"
)

// canonicalLSPImportPath keeps editor-side source discovery inside the same
// canonical logical-path domain as the parser and compiler resolver.
//
// Rules:
//   - rules/projects/modules.md — § 7 "Import paths"
//   - rules/projects/modules.md — § 9 "Import resolution"
func canonicalLSPImportPath(path string) bool {
	return modules.ValidateImportPath(path) == nil
}

// lspStandardLibraryIncludePaths resolves an arbitrary canonical stdlib module
// before project fallback. A module may be represented by one .sec file or by
// all .sec files in its module directory.
//
// Rules:
//   - rules/projects/modules.md — § 8 "Import roots"
//   - rules/projects/modules.md — § 9 "Import resolution"
//   - rules/tooling/lsp.md — "Project and source integration"
func lspStandardLibraryIncludePaths(root string, importPath string) ([]string, bool) {
	trimmed := strings.Trim(strings.TrimSuffix(importPath, ".sec"), "/")
	if trimmed == "" || strings.HasPrefix(trimmed, "platform/") {
		return nil, false
	}

	base := filepath.Join(root, "sec", "stdlib", filepath.FromSlash(trimmed))
	file := base + ".sec"
	if info, err := os.Stat(file); err == nil && !info.IsDir() {
		return []string{filepath.Clean(file)}, true
	}

	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		return nil, false
	}
	matches, err := filepath.Glob(filepath.Join(base, "*.sec"))
	if err != nil || len(matches) == 0 {
		return nil, false
	}
	sort.Strings(matches)
	return matches, true
}

// unresolvedImportError preserves the import spelling and exact path-token
// range in the shared structured diagnostic shape.
//
// Rules:
//   - rules/projects/modules.md — § 9 "Import resolution"
//   - rules/projects/modules.md — § 21 "Diagnostics"
func unresolvedImportError(stmt *ast.ImportStatement) sema.Error {
	token := stmt.PathToken
	if token.Line == 0 || token.Column == 0 {
		token = stmt.Token
	}
	endColumn := token.Column + utf8.RuneCountInString(token.Lexeme)
	return sema.Error{
		ID:        diagnostics.UnresolvedImport,
		Severity:  diagnostics.SeverityError,
		Message:   fmt.Sprintf("unresolved import %q", stmt.Path),
		Help:      "Verify the canonical import path and ensure the module exists in the selected standard-library, platform, or project source root.",
		File:      token.File,
		Line:      token.Line,
		Column:    token.Column,
		EndLine:   token.Line,
		EndColumn: endColumn,
	}
}
