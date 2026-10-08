package main

import (
	"os"
	"path/filepath"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/project"
	"sec/internal/sema"
)

// lspProgramEntryErrors checks the entry contract while editing a source of a
// command or firmware Target declared in the project manifest, so a missing,
// invalid, or duplicate main is shown before a build. Sources outside every
// declared Target, library Targets, and projects without a manifest are not
// checked: there the entry module is not designated.
//
// Rules:
//   - rules/compiler/initialization.md — § 20 "Target entry contracts", § 42 "Diagnostics"
//   - rules/tooling/lsp.md — diagnostics parity with the compiler
func lspProgramEntryErrors(analyzer *sema.Analyzer, program *ast.Program, sourcePath string, text string, overlay sourceOverlay) []sema.Error {
	if analyzer == nil || program == nil || sourcePath == "" {
		return nil
	}
	root := findProjectRoot(sourcePath)
	manifestPath := filepath.Join(root, ".sec", "sec.toml")
	manifest, ok := overlay.Sources[normalizedSourcePath(manifestPath)]
	if !ok {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil
		}
		manifest = string(data)
	}
	target, found := project.TargetForSource(project.ParseManifestTargets(manifest), root, sourcePath)
	if !found {
		return nil
	}
	kind := sema.ProgramTargetKind(target.Kind)
	if kind != sema.ProgramTargetCommand && kind != sema.ProgramTargetFirmware {
		return nil
	}
	module, anchor := "", lexer.Token{}
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.ModuleStatement); ok && declaration != nil &&
			normalizedSourcePath(declaration.Token.File) == normalizedSourcePath(sourcePath) {
			module, anchor = declaration.Path, declaration.Token
			break
		}
	}
	if module == "" {
		return nil
	}
	return analyzer.ValidateProgramEntry(kind, module, anchor)
}
