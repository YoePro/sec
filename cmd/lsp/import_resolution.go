package main

import (
	"path/filepath"

	"sec/internal/ast"
	"sec/internal/modules"
	"sec/internal/project"
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

// lspHardwareModuleContext supplies the same resolver-owned module identities
// as the compiler CLI, from the editor's own import resolution of the
// prepared program. The active document's directory is the anonymous root of
// sources outside a Project.
// Rules: MD-111 §§2,12,13(3); rules/projects/modules.md — §§2.2,9,23;
// rules/tooling/lsp.md — "Project and source integration".
func lspHardwareModuleContext(program *ast.Program, sourcePath string) sema.HardwareModuleContext {
	if program == nil || sourcePath == "" {
		return sema.HardwareModuleContext{}
	}
	resolver := project.NewSourceModuleResolver(findSecSourceRoot(sourcePath), filepath.Dir(sourcePath))
	return sema.HardwareModuleContextFor(program, resolver.ModuleIdentity, func(file, path string) []string {
		return sourceIncludePaths(path, file)
	})
}
