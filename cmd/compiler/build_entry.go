package main

import (
	"os"
	"path/filepath"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/project"
	"sec/internal/sema"
)

// validateBuildEntry checks the entry contract of the Target that sec build
// links. The Target kind comes from the nearest project manifest when the
// input file belongs to a declared Target; otherwise the linked executable is
// a command. The entry module is the input file's module.
//
// Rules:
//   - rules/compiler/initialization.md — § 20 "Target entry contracts", § 21, § 25, § 26
//   - rules/projects/projects.md — § 17 "Targets"
func validateBuildEntry(analyzed analyzedProgram, inputFile string) []sema.Error {
	if analyzed.Analyzer == nil || analyzed.Program == nil {
		return nil
	}
	module, anchor := entryModuleOf(analyzed.Program, inputFile)
	return analyzed.Analyzer.ValidateProgramEntry(buildTargetKind(inputFile), module, anchor)
}

// buildTargetKind resolves the kind of the Target owning a source file.
func buildTargetKind(sourceFile string) sema.ProgramTargetKind {
	manifest, found := findNearestProjectManifest(filepath.Dir(sourceFile))
	if !found {
		return sema.ProgramTargetCommand
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		return sema.ProgramTargetCommand
	}
	absolute, err := filepath.Abs(sourceFile)
	if err != nil {
		absolute = sourceFile
	}
	root := filepath.Dir(filepath.Dir(manifest))
	if target, ok := project.TargetForSource(project.ParseManifestTargets(string(data)), root, absolute); ok && target.Kind != "" {
		return sema.ProgramTargetKind(target.Kind)
	}
	return sema.ProgramTargetCommand
}

// entryModuleOf returns the module the input file declares and the
// declaration token that locates a missing-entry diagnostic. Core and
// imported sources assembled into the same program declare other modules.
func entryModuleOf(program *ast.Program, inputFile string) (string, lexer.Token) {
	for _, statement := range program.Statements {
		if module, ok := statement.(*ast.ModuleStatement); ok && module != nil && module.Token.File == inputFile {
			return module.Path, module.Token
		}
	}
	return "", lexer.Token{}
}
