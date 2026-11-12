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
	kind, found := lspSourceTargetKind(sourcePath, overlay)
	if !found {
		return nil
	}
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

// lspSourceTargetKind reads the kind of the manifest Target that owns a
// source, from the unsaved manifest overlay when present. It is the same
// selection the entry check uses, and it is applied before analysis so a
// library Target never acquires a program-entry root
// (initialization.md § 26).
func lspSourceTargetKind(sourcePath string, overlay sourceOverlay) (sema.ProgramTargetKind, bool) {
	if sourcePath == "" {
		return "", false
	}
	root := findProjectRoot(sourcePath)
	manifestPath := filepath.Join(root, ".sec", "sec.toml")
	manifest, ok := overlay.Sources[normalizedSourcePath(manifestPath)]
	if !ok {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return "", false
		}
		manifest = string(data)
	}
	target, found := project.TargetForSource(project.ParseManifestTargets(manifest), root, sourcePath)
	if !found || target.Kind == "" {
		return "", false
	}
	return sema.ProgramTargetKind(target.Kind), true
}
