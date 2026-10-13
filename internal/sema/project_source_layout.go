package sema

import (
	"path/filepath"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/project"
)

// ValidateProjectSourceLayout shares represented-source directory and import
// access validation between CLI and LSP before ordinary body analysis. Whole
// project discovery/validation remains a separate consumer of these rules.
// Rules: rules/projects/projects.md — §§14,15,48(2);
// rules/projects/modules.md — §13 "internal module access".
func ValidateProjectSourceLayout(program *ast.Program, manifestPath string) []Error {
	if program == nil {
		return nil
	}
	absolute, err := filepath.Abs(manifestPath)
	if err != nil {
		return nil
	}
	root := filepath.Dir(filepath.Dir(absolute))
	var failures []Error
	for _, statement := range program.Statements {
		var token lexer.Token
		var failure error
		switch declaration := statement.(type) {
		case *ast.ModuleStatement:
			token = declaration.NameToken
			if token.Line == 0 {
				token = declaration.Token
			}
			if token.File == "" {
				continue
			}
			_, failure = project.SourceLocation(root, token.File)
		case *ast.ImportStatement:
			token = declaration.PathToken
			if token.Line == 0 {
				token = declaration.Token
			}
			if token.File == "" {
				continue
			}
			failure = project.ProjectImportAccess(root, token.File, declaration.Path)
		default:
			continue
		}
		provenance := program.SourceProvenance[token.File]
		if provenance == ast.SourceCore || provenance == ast.SourceStdlib || provenance == ast.SourcePlatform {
			continue
		}
		if failure == nil {
			continue
		}
		endLine, endColumn := token.EndPosition()
		failures = append(failures, Error{ID: diagnostics.ProjectSourceLayout, Severity: diagnostics.SeverityError,
			Message: failure.Error(), Help: "Keep source modules in ordinary project directories or cmd child directories. Import an internal module only from its owning subtree; bin and .sec hold outputs and metadata.",
			File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn})
	}
	return failures
}
