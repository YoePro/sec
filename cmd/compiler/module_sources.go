package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// assembleCLIModuleSources adds unselected siblings of directory modules before
// semantic analysis. Legacy standalone snippets outside the directory/module
// naming model remain single-file inputs; project roots use their manifest.
// Rules: rules/projects/modules.md — "Source directory and module membership",
// "Module declaration"; rules/projects/projects.md — §15.4 `.sec` is
// metadata storage, not a source file. Syntax errors in siblings still block compilation.
func assembleCLIModuleSources(program *ast.Program, target CompilerTarget) diagnosticSummary {
	summary := diagnosticSummary{}
	seen := map[string]bool{}
	modules := map[string]string{}
	for _, stmt := range program.Statements {
		decl, ok := stmt.(*ast.ModuleStatement)
		if !ok || decl.Token.File == "" {
			continue
		}
		path, err := filepath.Abs(decl.Token.File)
		if err != nil {
			continue
		}
		seen[path] = true
		dir := filepath.Dir(path)
		_, manifestErr := os.Stat(filepath.Join(dir, ".sec", "sec.toml"))
		entry := filepath.Base(filepath.Dir(dir)) == "cmd" && decl.Path == "main" &&
			fileExists(filepath.Join(filepath.Dir(filepath.Dir(dir)), ".sec", "sec.toml"))
		if filepath.Base(dir) == decl.Path || manifestErr == nil || entry {
			modules[dir] = decl.Path
		}
	}
	for _, err := range cliProjectModuleErrors(program) {
		printSemaError(os.Stderr, err)
		summary.Errors++
	}
	dirs := make([]string, 0, len(modules))
	for dir := range modules {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.sec"))
		for _, file := range files {
			if info, err := os.Stat(file); err == nil && info.IsDir() {
				continue
			}
			if seen[file] || strings.HasSuffix(file, "_test.sec") || !sourcePathMatchesTarget(file, target) {
				continue
			}
			seen[file] = true
			source, counts := parseSourceFileWithDiagnostics(file)
			for _, err := range cliProjectModuleErrors(source.Program) {
				printSemaError(os.Stderr, err)
				summary.Errors++
			}
			if programModulePath(source.Program) != modules[dir] {
				continue
			}
			if validateProgramTarget(source.Program, target) != nil {
				continue
			}
			summary.Errors += counts.Errors
			summary.Warnings += counts.Warnings
			printParserWarningsForFile(file, source.Warnings)
			cliDiagnostics.parserDiagnostics(file, source.Diagnostics, source.Errors)
			program.Statements = append(program.Statements, source.Program.Statements...)
		}
	}
	return summary
}

// cliProjectModuleErrors validates root naming, reserved source directories
// and internal import access before name resolution, including omitted siblings.
// Rules: rules/projects/projects.md — §§14–15; rules/projects/modules.md — §§4,13.
func cliProjectModuleErrors(program *ast.Program) []sema.Error {
	if program == nil {
		return nil
	}
	manifests := map[string]bool{}
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.ModuleStatement); ok && declaration.Token.File != "" {
			if path, found := findNearestProjectManifest(filepath.Dir(declaration.Token.File)); found {
				manifests[path] = true
			}
		}
	}
	paths := make([]string, 0, len(manifests))
	for path := range manifests {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var errors []sema.Error
	for _, path := range paths {
		if text, err := os.ReadFile(path); err == nil {
			errors = append(errors, sema.ValidateProjectRootModules(program, path, string(text))...)
			errors = append(errors, sema.ValidateProjectSourceLayout(program, path)...)
		}
	}
	return errors
}
