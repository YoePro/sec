package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sourceimports"
)

// UnresolvedSourceImport reports failed discovery at the exact import token.
// Rules: rules/projects/modules.md — §§8,9,21.
func UnresolvedSourceImport(stmt *ast.ImportStatement) Error {
	token := stmt.PathToken
	if token.Line == 0 || token.Column == 0 {
		token = stmt.Token
	}
	endLine, endColumn := token.EndPosition()
	return Error{ID: diagnostics.UnresolvedImport, Severity: diagnostics.SeverityError,
		Message: fmt.Sprintf("unresolved import %q", stmt.Path),
		Help:    "Verify the canonical import path and ensure the module exists in the selected standard-library, platform, or project source root.",
		File:    token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn}
}

// ImportedModuleConflicts reports a failed directory-module import where the
// importer sees it, with the inconsistent declaration as a related location.
// Rules: rules/projects/modules.md — §§3,9,21.
func ImportedModuleConflicts(stmt *ast.ImportStatement, paths []string, read func(string) ([]byte, error)) []Error {
	var errors []Error
	for _, conflict := range sourceimports.ModuleConflicts(paths, read) {
		issue := UnresolvedSourceImport(stmt)
		issue.ID = diagnostics.ImportedModuleMismatch
		issue.Message = fmt.Sprintf("cannot import %q: %s declares module %s, but the source directory declares module %s", stmt.Path, conflict.Actual.File, conflict.Actual.Lexeme, conflict.Expected)
		issue.Help = "All production files in one module directory must declare the same module name. Correct or relocate the inconsistent source file before importing this module."
		issue.PreviousFile = conflict.Actual.File
		issue.PreviousLine = conflict.Actual.Line
		issue.PreviousColumn = conflict.Actual.Column
		issue.RelatedLabel = "inconsistent module declaration"
		errors = append(errors, issue)
	}
	return errors
}
