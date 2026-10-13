package main

import (
	"os"

	"sec/internal/ast"
)

// resolveStdlibImportsWithSources loads the source closure for one target.
// Rules: rules/projects/modules.md — §§3,8–9,13.
func resolveStdlibImportsWithSources(program *ast.Program, target CompilerTarget, sourceFiles []string) {
	// TODO: Replace this source-level stdlib inclusion with compiled library metadata/artifacts.
	seen := map[string]bool{}
	resolveStdlibImportsInto(program, target, seen, sourceFiles)
}

// resolveStdlibImportsInto validates source/import layout before recursive
// loading, qualification and declaration visibility.
// Rules: rules/projects/projects.md — §15; rules/projects/modules.md — §§9,13.
func resolveStdlibImportsInto(program *ast.Program, target CompilerTarget, seen map[string]bool, sourceFiles []string) {
	if issues := cliProjectModuleErrors(program); len(issues) != 0 {
		for _, issue := range issues {
			printSemaError(os.Stderr, issue)
		}
		printDiagnosticSummary(diagnosticSummary{Errors: len(issues)})
		exitCLI(3)
	}
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok {
			continue
		}
		sourcePaths, ok := sourceIncludePathsWithSources(importStmt.Path, target, sourceFiles)
		if !ok {
			continue
		}
		imported := &ast.Program{}
		module := ""
		for _, sourcePath := range sourcePaths {
			if seen[sourcePath] {
				if module == "" {
					module = programModulePath(parseSourceInclude(sourcePath, target))
				}
				continue
			}
			seen[sourcePath] = true
			sourceProgram := parseSourceInclude(sourcePath, target)
			if module == "" {
				module = programModulePath(sourceProgram)
			}
			imported.Statements = append(imported.Statements, sourceProgram.Statements...)
		}
		if len(imported.Statements) == 0 {
			rewriteImportQualifier(program, importQualifier(importStmt), module)
			continue
		}

		resolveStdlibImportsInto(imported, target, seen, sourcePaths)
		if module == "" {
			module = programModulePath(imported)
		}
		rewriteImportQualifier(program, importQualifier(importStmt), module)
		qualifyImportedModule(imported, module)
		program.Statements = append(program.Statements, imported.Statements...)
	}
}
