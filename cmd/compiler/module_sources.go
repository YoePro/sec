package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/ast"
)

// assembleCLIModuleSources adds unselected siblings of directory modules before
// semantic analysis. Legacy standalone snippets outside the directory/module
// naming model remain single-file inputs; project roots use their manifest.
// Rules: rules/projects/modules.md — "Source directory and module membership",
// "Module declaration". Syntax errors in siblings still block compilation.
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
		if filepath.Base(dir) == decl.Path || manifestErr == nil {
			modules[dir] = decl.Path
		}
	}
	dirs := make([]string, 0, len(modules))
	for dir := range modules {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.sec"))
		for _, file := range files {
			if seen[file] || strings.HasSuffix(file, "_test.sec") || !sourcePathMatchesTarget(file, target) {
				continue
			}
			seen[file] = true
			source, counts := parseSourceFileWithDiagnostics(file)
			if programModulePath(source.Program) != modules[dir] {
				continue
			}
			if validateProgramTarget(source.Program, target) != nil {
				continue
			}
			summary.Errors += counts.Errors
			summary.Warnings += counts.Warnings
			printParserWarningsForFile(file, source.Warnings)
			for _, message := range source.Errors {
				fmt.Fprintf(os.Stderr, "%s: parse error: %s\n", file, message)
			}
			program.Statements = append(program.Statements, source.Program.Statements...)
		}
	}
	return summary
}
