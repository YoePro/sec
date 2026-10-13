package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/project"
)

// ValidateProjectVersion produces registered manifest occurrences from the
// shared Project version reader for compiler and LSP consumers.
// Rules: rules/projects/projects.md — §§8–11,48(1–3); diagnostics.md — §9.
func ValidateProjectVersion(path, text string) []Error {
	var result []Error
	for _, issue := range project.ValidateVersionSnapshot(text) {
		result = append(result, Error{ID: diagnostics.ProjectInvalidVersion, Severity: diagnostics.SeverityError,
			Message: issue.Message, Help: "Declare exactly one [version] table with major, minor, revision and build in 0..9223372036854775807 and a format using defined placeholders. Target overrides must resolve with the inherited Project version.",
			File: path, Line: issue.Line, Column: issue.Column, EndLine: issue.Line, EndColumn: issue.EndColumn})
	}
	return result
}
