package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/project"
)

// ValidateProjectTargetSources adapts the shared manifest source-boundary
// validator to registered occurrences for CLI and unsaved editor snapshots.
// Rules: rules/tooling/testing.md — §§8.1,25.2,26.5;
// rules/tooling/diagnostics.md — §§8–9,12;
// rules/projects/projects.md — §48.
func ValidateProjectTargetSources(path, text string) []Error {
	var result []Error
	for _, issue := range project.ValidateTargetSourcesSnapshot(text) {
		result = append(result, Error{
			ID: diagnostics.ProductionDependsOnTest, Severity: diagnostics.SeverityError,
			Message: issue.Message, Help: "Move the production Target source outside the project-root tests/ tree, or declare kind = \"test\" for an intentionally test-only product.",
			File: path, Line: issue.Line, Column: issue.Column, EndLine: issue.Line, EndColumn: issue.EndColumn,
		})
	}
	return result
}
