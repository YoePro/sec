package project

import (
	"fmt"
	"strings"
	"testing"
)

// TestManifestVersionUnsupportedForms reports reader capabilities separately
// from invalid VersionNumber values, for project and inherited Target fields.
// Rules: rules/projects/projects.md — §§8(3),9–11,48(2).
func TestManifestVersionUnsupportedForms(t *testing.T) {
	const base = "[version]\nmajor = 0\nminor = 1\nrevision = 0\nbuild = 0\nformat = \"[major].[minor].[revision].[build]\"\n"
	for _, test := range []struct{ raw, want string }{
		{"0x10", "hexadecimal TOML integers"}, {"0o10", "octal TOML integers"}, {"0b10", "binary TOML integers"},
	} {
		for _, target := range []bool{false, true} {
			source := strings.Replace(base, "build = 0", "build = "+test.raw, 1)
			line := 5
			if target {
				source = base + "[target.app.version]\nbuild = " + test.raw + "\n"
				line = 8
			}
			_, errors := ParseManifestVersions(source)
			if len(errors) != 1 || errors[0].Line != line || !strings.Contains(errors[0].Message, test.want) || !strings.Contains(errors[0].Message, "use a decimal integer") {
				t.Fatal(source, errors)
			}
		}
	}
	for _, delimiter := range []string{"\"\"\"", "'''"} {
		for _, target := range []bool{false, true} {
			for _, multiline := range []bool{false, true} {
				value := delimiter + "[major]" + delimiter
				if multiline {
					value = delimiter + "\n[target.fake.version]\nbuild = 99\n# text inside the rejected string\n" + delimiter
				}
				source := strings.Replace(base, `"[major].[minor].[revision].[build]"`, value, 1)
				line := 6
				if target {
					source = base + "[target.app.version]\nformat = " + value + "\n"
					line = 8
				}
				source += "[target.real.version]\nbuild = 12\n"
				result, errors := ParseManifestVersions(source)
				if len(errors) != 1 || errors[0].Line != line || !strings.Contains(errors[0].Message, "multiline TOML strings are not supported") || !strings.Contains(errors[0].Message, "single-line") {
					t.Fatal(fmt.Sprintf("%q", source), errors)
				}
				if _, found := result.TargetOverrides["fake"]; found {
					t.Fatal("string contents became a Target", result)
				}
				if real := result.TargetOverrides["real"].Build; real == nil || *real != 12 {
					t.Fatal("failed to recover to next real table", result)
				}
			}
		}
	}
}

// TestUnsupportedMultilineRecovery retains escaped delimiter text and never
// reinterprets an unterminated literal as another version table.
// Rules: rules/projects/projects.md — §§48(2),49(2).
func TestUnsupportedMultilineRecovery(t *testing.T) {
	source := "[version]\nmajor=0\nminor=1\nrevision=0\nbuild=0\nformat=\"\"\"\n\\\"\"\"\n[target.fake.version]\nbuild=99\n\"\"\"\n[target.real.version]\nbuild=12\n"
	result, errors := ParseManifestVersions(source)
	if len(errors) != 1 {
		t.Fatal(errors)
	}
	if _, found := result.TargetOverrides["fake"]; found {
		t.Fatal(result)
	}
	if result.TargetOverrides["real"].Build == nil {
		t.Fatal(result)
	}
	source = "[version]\nmajor=0\nminor=1\nrevision=0\nbuild=0\nformat='''\n[target.fake.version]\nbuild=99\n"
	result, errors = ParseManifestVersions(source)
	if len(errors) != 1 || len(result.TargetOverrides) != 0 {
		t.Fatal(result, errors)
	}
}
