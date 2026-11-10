package main

import (
	"bytes"
	"os"
	"path/filepath"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/project"
	"sec/internal/sema"
	"sec/internal/sourceimports"
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
	identities := map[[2]string]*ast.ImportStatement{}
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok {
			continue
		}

		sourcePaths, ok := sourceIncludePathsWithSources(importStmt.Path, target, sourceFiles)
		if !ok {
			printSemaError(os.Stderr, sema.UnresolvedSourceImport(importStmt))
			printDiagnosticSummary(diagnosticSummary{Errors: 1})
			exitCLI(3)
		}
		if issues := sema.ImportedModuleConflicts(importStmt, sourcePaths, os.ReadFile); len(issues) != 0 {
			for _, issue := range issues {
				printSemaError(os.Stderr, issue)
			}
			printDiagnosticSummary(diagnosticSummary{Errors: len(issues)})
			exitCLI(3)
		}
		if issues := sema.ImportIdentityIssues(importStmt, "", "", sourcePaths, identities); len(issues) != 0 {
			for _, issue := range issues {
				printSemaError(os.Stderr, issue)
			}
			printDiagnosticSummary(diagnosticSummary{Errors: len(issues)})
			exitCLI(3)
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

// compilerStandardLibrarySources shares canonical library discovery with LSP
// and applies the selected compiler target before parsing the source closure.
// Rules: rules/projects/modules.md — §§3,8,9;
// rules/platform/platform_model.md — source selection by target.
func compilerStandardLibrarySources(path string, target CompilerTarget, sources []string) ([]string, bool) {
	paths, ok := sourceimports.StandardLibrarySources(findCompilerSourceRoot(sources), path)
	if !ok {
		return nil, false
	}
	selected := []string{}
	for _, path := range paths {
		if sourcePathMatchesTarget(path, target) {
			if data, err := os.ReadFile(path); err == nil && bytes.Contains(data, []byte("#target")) {
				program := parser.New(lexer.NewWithFile(string(data), path)).ParseProgram()
				if validateProgramTarget(program, target) != nil {
					continue
				}
			}
			selected = append(selected, path)
		}
	}
	return selected, true
}

// cliHardwareModuleContext gives Sema the resolver-owned identity of every
// assembled source file and the modules its imports select, using the same
// import resolution that loaded the sources. The entry directory is the
// anonymous root of sources outside a Project.
// Rules: MD-111 §§2,12; rules/projects/modules.md — §§2.2,8–9;
// rules/projects/projects.md — §13(9).
func cliHardwareModuleContext(program *ast.Program, target CompilerTarget, sourceFiles []string) sema.HardwareModuleContext {
	entry := "."
	if len(sourceFiles) != 0 {
		entry = filepath.Dir(sourceFiles[0])
	}
	resolver := project.NewSourceModuleResolver(findCompilerSourceRoot(sourceFiles), entry)
	return sema.HardwareModuleContextFor(program, resolver.ModuleIdentity, func(file, path string) []string {
		paths, _ := sourceIncludePathsWithSources(path, target, []string{file})
		return paths
	})
}
