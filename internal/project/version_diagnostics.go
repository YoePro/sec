package project

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// VersionDiagnostic locates a version failure in a retained manifest snapshot.
// Columns count Unicode scalars; protocol conversion belongs to tooling.
// Rules: rules/projects/projects.md — §§8–11,48(1–3).
type VersionDiagnostic struct {
	Message                 string
	Line, Column, EndColumn int
}

// ValidateVersionSnapshot validates TOML, required version fields and Target
// inheritance independently of unrelated project identity fields. Explicit
// unsupported-form errors remain owned by the shared version reader.
// Rules: rules/projects/projects.md — §§5(2),8–11,48(1–3).
func ValidateVersionSnapshot(text string) []VersionDiagnostic {
	_, failures := ParseManifestVersions(text)
	if document, err := ParseManifestDocument(text); err != nil {
		var syntax toml.ParseError
		if errors.As(err, &syntax) {
			failures = []ManifestError{{Line: syntax.Position.Line, Message: "invalid manifest TOML: " + syntax.Message}}
		}
	} else if _, err := document.Table("version"); err != nil {
		failures = []ManifestError{{Line: manifestKeyLine(text, []string{"version"}), Message: "version must be a TOML table"}}
	} else if targets, err := document.Table("target"); err == nil {
		for name, value := range targets {
			if table, ok := value.(map[string]any); ok {
				if version, exists := table["version"]; exists {
					if _, ok := version.(map[string]any); !ok {
						failures = append(failures, ManifestError{Line: manifestKeyLine(text, []string{"target", name, "version"}), Message: "target." + name + ".version must be a TOML table"})
					}
				}
			}
		}
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	result := make([]VersionDiagnostic, 0, len(failures))
	for _, failure := range failures {
		line := failure.Line
		if line <= 0 && strings.HasPrefix(failure.Message, "target ") {
			fields := strings.Fields(failure.Message)
			if len(fields) > 1 {
				line = manifestKeyLine(text, []string{"target", fields[1], "version"})
			}
		}
		if line <= 0 || line > len(lines) {
			line = 1
		}
		raw := lines[line-1]
		body := strings.TrimSpace(stripComment(raw))
		column := utf8.RuneCountInString(raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]) + 1
		result = append(result, VersionDiagnostic{failure.Message, line, column, column + utf8.RuneCountInString(body)})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		return result[i].Message < result[j].Message
	})
	return result
}
