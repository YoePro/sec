package main

import (
	"sec/internal/ast"
	"sec/internal/modules"
	"sec/internal/sema"
	"sec/internal/sourceimports"
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
	return sourceimports.StandardLibrarySources(root, importPath)
}

// unresolvedImportError preserves the import spelling and exact path-token
// range in the shared structured diagnostic shape.
//
// Rules:
//   - rules/projects/modules.md — § 9 "Import resolution"
//   - rules/projects/modules.md — § 21 "Diagnostics"
func unresolvedImportError(stmt *ast.ImportStatement) sema.Error {
	return sema.UnresolvedSourceImport(stmt)
}
