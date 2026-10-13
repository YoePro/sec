package sema

import (
	"fmt"
	"path/filepath"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/project"
)

// ValidateProjectRootModules checks source declarations against the canonical
// root ModuleName resolution shared by CLI/LSP. Other invalid manifest fields
// remain the responsibility of discovered-project validation, not this check.
// Rules: rules/projects/projects.md — §§7(7–8),14(1–9),48(2),53(8);
// rules/tooling/diagnostics.md — §§5,9,21.
func ValidateProjectRootModules(program *ast.Program, manifestPath, text string) []Error {
	if program == nil {
		return nil
	}
	manifest, failures := project.ParseManifest(manifestPath, text)
	expected, message := "", ""
	if manifest != nil {
		var err error
		expected, err = manifest.RootModuleName()
		if err != nil {
			message = err.Error()
		}
	} else {
		// Explicitly malformed overrides are owned by this feature. Legacy partial
		// manifests expose no resolved project, and retain their existing behavior.
		for _, failure := range failures {
			if strings.HasPrefix(failure.Message, "project.module") || failure.Message == "duplicate project field module" {
				message = failure.Message
				break
			}
		}
		if message == "" {
			return nil
		}
	}
	absolute, err := filepath.Abs(manifestPath)
	if err != nil {
		return nil
	}
	root := filepath.Dir(filepath.Dir(absolute))
	var errors []Error
	for _, statement := range program.Statements {
		declaration, ok := statement.(*ast.ModuleStatement)
		if !ok || declaration.Token.File == "" {
			continue
		}
		source, err := filepath.Abs(declaration.Token.File)
		if err != nil || filepath.Dir(source) != root {
			continue
		}
		problem := message
		if problem == "" && declaration.Path != expected {
			problem = fmt.Sprintf("root module declares %q; expected %q from the project root-module contract", declaration.Path, expected)
		}
		if problem == "" {
			continue
		}
		token := declaration.NameToken
		if token.Line == 0 {
			token = declaration.Token
		}
		endLine, endColumn := token.EndPosition()
		errors = append(errors, Error{ID: diagnostics.ProjectRootModuleMismatch, Severity: diagnostics.SeverityError,
			Message: problem, Help: "Use the expected root module name, or set [project].module to the intended root library ModuleName. A root command uses module main.",
			File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn})
	}
	return errors
}
