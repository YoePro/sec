package main

import (
	"sec/internal/ast"
	lspserver "sec/internal/lsp/server"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// resolveSourceImports loads the compiler source closure for the host plan.
// Rules: rules/projects/modules.md — §§3,9; rules/tooling/lsp.md — "Project and source integration".
func resolveSourceImports(program *ast.Program, seen map[string]bool, sourceFile string, overlays ...sourceOverlay) []sema.Error {
	return resolveSourceImportsForTarget(program, seen, sourceFile, platformtarget.Host(), overlays...)
}

// resolveSourceImportsForTarget includes the sources of every imported module
// that belong to target; a file whose `#target` selects another platform is
// left out, as the compiler does. Imported modules include unsaved siblings.
// Rules: rules/projects/modules.md — §3 "Source directory and module membership";
// rules/errors/panic.md — §28(8); rules/tooling/lsp.md — "Snapshots".
func resolveSourceImportsForTarget(program *ast.Program, seen map[string]bool, sourceFile string, target platformtarget.Target, overlays ...sourceOverlay) []sema.Error {
	overlay := firstSourceOverlay(overlays)
	issues := lspProjectSourceLayoutErrors(program, sourceFile, overlay)
	if len(issues) != 0 {
		return issues
	}
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok || importStmt == nil {
			continue
		}
		if !canonicalLSPImportPath(importStmt.Path) {
			continue
		}
		sourcePaths := sourceIncludePaths(importStmt.Path, sourceFile)
		if len(sourcePaths) == 0 {
			issues = append(issues, unresolvedImportError(importStmt))
			continue
		}
		importedStatements := []ast.Statement{}
		module := ""
		resolved := false
		for _, sourcePath := range sourcePaths {
			if seen[sourcePath] {
				if imported, parsed := parseSourceInclude(sourcePath, overlay); parsed {
					resolved = true
					rewriteImportQualifier(program, importQualifier(importStmt), programModulePath(imported))
				}
				continue
			}
			seen[sourcePath] = true

			imported, ok := parseSourceInclude(sourcePath, overlay)
			if !ok {
				continue
			}
			resolved = true
			if !lspserver.ProgramMatchesTarget(imported, target) {
				continue
			}
			lspserver.AssembleModuleForTarget(imported, sourcePath, overlay.Sources, target)
			for _, declaration := range imported.Statements {
				if token, known := lspStatementTokenForSource(declaration); known && token.File != "" {
					seen[token.File] = true
				}
			}
			issues = append(issues, resolveSourceImportsForTarget(imported, seen, sourcePath, target, overlay)...)
			if module == "" {
				module = programModulePath(imported)
			}
			importedStatements = append(importedStatements, imported.Statements...)
		}
		if !resolved {
			issues = append(issues, unresolvedImportError(importStmt))
			continue
		}
		if module == "" || len(importedStatements) == 0 {
			continue
		}
		rewriteImportQualifier(program, importQualifier(importStmt), module)
		imported := &ast.Program{Statements: importedStatements}
		qualifyImportedModule(imported, module)
		program.Statements = append(program.Statements, imported.Statements...)
	}
	return issues
}
