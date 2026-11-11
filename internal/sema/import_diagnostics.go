package sema

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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

// ImportIdentityIssues rejects an import whose resolved canonical module
// identity (the resolved set of module source files) contains the importing
// file itself, or was already imported by the same source file under any
// spelling or alias. previous maps importer file and module identity to the
// first import for one resolution pass. importerFallback names the analyzed
// source file when the import token carries no file, and importerModule its
// declared module when the resolver leaves that file out of its own
// directory module (an editor buffer); then a same-directory import whose
// final path segment names the importer's module is also a self-import.
// Rules: rules/projects/modules.md — § 14 "Self-imports, duplicate imports, and cycles", § 21.
func ImportIdentityIssues(stmt *ast.ImportStatement, importerFallback, importerModule string, sourcePaths []string, previous map[[2]string]*ast.ImportStatement) []Error {
	if stmt == nil || len(sourcePaths) == 0 {
		return nil
	}
	importerFile := stmt.Token.File
	if importerFile == "" {
		importerFile = importerFallback
	}
	importer := canonicalSourcePath(importerFile)
	members := make([]string, 0, len(sourcePaths))
	for _, path := range sourcePaths {
		members = append(members, canonicalSourcePath(path))
	}
	sort.Strings(members)
	sameDirectoryModule := false
	if importerModule != "" && importerFile != "" && filepath.Dir(members[0]) == filepath.Dir(importer) {
		segments := strings.Split(stmt.Path, "/")
		sameDirectoryModule = segments[len(segments)-1] == importerModule
	}
	for _, member := range members {
		if importerFile != "" && member == importer || sameDirectoryModule {
			issue := UnresolvedSourceImport(stmt)
			issue.ID = diagnostics.SelfImport
			issue.Message = fmt.Sprintf("module cannot import itself through %q", stmt.Path)
			issue.Help = "Files of one module share one namespace and need no import; remove this import."
			return []Error{issue}
		}
	}
	key := [2]string{importer, strings.Join(members, "\x00")}
	if first, seen := previous[key]; seen {
		issue := UnresolvedSourceImport(stmt)
		issue.ID = diagnostics.DuplicateImport
		issue.Message = fmt.Sprintf("module %q is already imported in this source file", stmt.Path)
		issue.Help = "Import each module once per source file; use the first import's qualifier, or give that single import the alias you need."
		token := first.PathToken
		if token.Line == 0 {
			token = first.Token
		}
		issue.PreviousFile, issue.PreviousLine, issue.PreviousColumn = token.File, token.Line, token.Column
		issue.RelatedLabel = "first import of this module"
		return []Error{issue}
	}
	// Distinct modules may share a ModuleName, but each import binding in one
	// source file needs a unique local qualifier (modules.md § 11).
	qualifier := ImportLocalQualifier(stmt)
	qualifierKey := [2]string{importer, "qualifier\x00" + qualifier}
	if first, seen := previous[qualifierKey]; seen && qualifier != "" {
		issue := UnresolvedSourceImport(stmt)
		issue.ID = diagnostics.ImportQualifierCollision
		issue.Message = fmt.Sprintf("import %q binds qualifier %s, which an earlier import in this source file already binds", stmt.Path, qualifier)
		issue.Help = fmt.Sprintf("Give one of the imports a distinct alias, for example `import %sAlias %q`.", qualifier, stmt.Path)
		token := first.PathToken
		if token.Line == 0 {
			token = first.Token
		}
		issue.PreviousFile, issue.PreviousLine, issue.PreviousColumn = token.File, token.Line, token.Column
		issue.RelatedLabel = "earlier import binding " + qualifier
		return []Error{issue}
	}
	previous[key] = stmt
	if qualifier != "" {
		previous[qualifierKey] = stmt
	}
	return nil
}

// ImportLocalQualifier is the file-local binding an import introduces: its
// alias, otherwise the final segment of the canonical import path.
// Rules: rules/projects/modules.md — §§ 10–11.
func ImportLocalQualifier(stmt *ast.ImportStatement) string {
	if stmt == nil {
		return ""
	}
	if stmt.Alias != "" {
		return stmt.Alias
	}
	trimmed := strings.Trim(strings.TrimSuffix(stmt.Path, ".sec"), "/")
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[index+1:]
	}
	return trimmed
}

func canonicalSourcePath(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}
