package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/project"
)

// ValidateProjectBoundary transports shared read-only discovery failures and
// nested-manifest occurrences to CLI and LSP with one mandatory diagnostic ID.
// Rules: rules/projects/projects.md — §§4,48(1–3).
func ValidateProjectBoundary(path string, overlays map[string]string) []Error {
	violations, err := project.ValidateBoundary(path, overlays)
	if err != nil {
		return []Error{{ID: diagnostics.ProjectInvalidBoundary, Severity: diagnostics.SeverityError,
			Message: "cannot validate project boundary: " + err.Error(), Help: "Make the project tree and its manifests readable, then retry project discovery.",
			File: path, Line: 1, Column: 1, EndLine: 1, EndColumn: 1}}
	}
	result := make([]Error, 0, len(violations))
	for _, violation := range violations {
		result = append(result, Error{ID: diagnostics.ProjectInvalidBoundary, Severity: diagnostics.SeverityError,
			Message: fmt.Sprintf("nested project manifest %s is governed by outer manifest %s; nested Sec projects are not supported", violation.Manifest, violation.GoverningManifest),
			Help:    "Keep one governing .sec/sec.toml per project. Place an independent project outside the outer project tree; ordinary child modules do not need their own manifest.",
			File:    violation.Manifest, Line: violation.Line, Column: 1, EndLine: violation.Line, EndColumn: 10,
			PreviousFile: violation.GoverningManifest, PreviousLine: 1, PreviousColumn: 1, RelatedLabel: "governing outer project manifest"})
	}
	return result
}
