package project

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// TargetSourceDiagnostic locates a test-only source selection in a retained
// manifest snapshot. Columns count Unicode scalars, as in version diagnostics.
// Rules: rules/tooling/testing.md — §8.1;
// rules/projects/projects.md — §§17–18,48.
type TargetSourceDiagnostic struct {
	Message                 string
	Line, Column, EndColumn int
}

// ValidateTargetSourcesSnapshot enforces the project-root tests/ boundary on
// logical Target source paths before building a production plan. Normalization
// uses project-relative slash paths independently of the compiler host; a
// nested directory named tests is ordinary. Syntax errors stay with TOML
// validation; no invalid snapshot acquires a resolved manifest from this query.
// Rules: rules/tooling/testing.md — §§4.3,8.1,26.5;
// rules/projects/projects.md — §§17(3–5),18,48(1–3).
func ValidateTargetSourcesSnapshot(text string) []TargetSourceDiagnostic {
	var result []TargetSourceDiagnostic
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, target := range ParseManifestTargets(text) {
		source := path.Clean(target.Source)
		if target.Kind == "test" || source != "tests" && !strings.HasPrefix(source, "tests/") {
			continue
		}
		line := manifestKeyLine(text, []string{"target", target.Name, "source"})
		if line < 1 || line > len(lines) {
			line = 1
		}
		raw := lines[line-1]
		column := utf8.RuneCountInString(raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]) + 1
		body := strings.TrimSpace(stripComment(raw))
		result = append(result, TargetSourceDiagnostic{
			Message: fmt.Sprintf("target %s selects test-only source %q; only kind = \"test\" may select the project-root tests/ tree", target.Name, target.Source),
			Line:    line, Column: column, EndColumn: column + utf8.RuneCountInString(body),
		})
	}
	return result
}
