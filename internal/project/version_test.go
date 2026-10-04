package project

import (
	"strings"
	"testing"
)

// The structured version renders from its fields through the five
// placeholders, Target overrides replace only present fields, and an empty
// Target userdefined clears the inherited component.
//
// Rules:
//   - rules/projects/projects.md — § 8(10)–(11), § 9(6)–(7), § 10, § 11, § 12
func TestManifestVersionsRenderAndResolve(t *testing.T) {
	manifest := `[project]
name = "SEC Compiler Suite"
uuid = "550e8400-e29b-41d4-a716-446655440000"

[version]
major = 1
minor = 0
revision = 3
build = 123 # visible build number
userdefined = "alpha"
format = "[major].[minor].[revision]-[userdefined].[build]"

[target.sec]
kind = "command"

[target.sec.version]
build = 421

[target.lsp.version]
major = 0
minor = 8
revision = 2
build = 97
userdefined = "beta"
format = "[major].[minor].[revision]-[userdefined].[build]"

[target.plain.version]
userdefined = ""
format = '[major].[minor].[revision]+[build]'
`
	versions, errors := ParseManifestVersions(manifest)
	if len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	for name, want := range map[string]string{"": "1.0.3-alpha.123", "sec": "1.0.3-alpha.421", "lsp": "0.8.2-beta.97", "plain": "1.0.3+123"} {
		version := versions.Project
		if name != "" {
			version = version.Resolve(versions.TargetOverrides[name])
		}
		got, err := version.Render()
		if err != nil || got != want {
			t.Errorf("%q renders %q, %v; want %q", name, got, err, want)
		}
	}
}

// Manifest validation reports a missing or duplicate table, missing fields,
// out-of-range numbers, invalid userdefined values, unknown placeholders, a
// format that needs a missing userdefined component, and a Target that
// clears userdefined under an inherited format that references it.
//
// Rules:
//   - rules/projects/projects.md — § 8(1)–(3), § 9(3), § 10(6)–(7), (10), § 11(9), § 48
func TestManifestVersionsValidation(t *testing.T) {
	const valid = "[version]\nmajor = 0\nminor = 1\nrevision = 0\nbuild = 0\nformat = \"[major].[minor].[revision].[build]\"\n"
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{name: "missing table", manifest: "[project]\nname = \"x\"\n", want: "no [version] table"},
		{name: "duplicate table", manifest: valid + valid, want: "exactly one [version]"},
		{name: "missing field", manifest: "[version]\nmajor = 0\nminor = 1\nrevision = 0\nformat = \"[major]\"\n", want: "missing the required field build"},
		{name: "negative number", manifest: strings.Replace(valid, "build = 0", "build = -1", 1), want: "negative"},
		{name: "number too large", manifest: strings.Replace(valid, "build = 0", "build = 9223372036854775808", 1), want: "exceeds"},
		{name: "leading zero", manifest: strings.Replace(valid, "build = 0", "build = 07", 1), want: "leading zero"},
		{name: "invalid userdefined", manifest: valid + "userdefined = \"-beta\"\n", want: "must match"},
		{name: "empty project userdefined", manifest: valid + "userdefined = \"\"\n", want: "omit it"},
		{name: "unknown placeholder", manifest: strings.Replace(valid, "[build]\"", "[patch]\"", 1), want: "unknown placeholder [patch]"},
		{name: "format needs userdefined", manifest: strings.Replace(valid, "[build]\"", "[userdefined]\"", 1), want: "no user-defined component"},
		{name: "target clears userdefined", manifest: strings.Replace(valid, "[build]\"", "[userdefined]\"", 1) + "userdefined = \"rc1\"\n\n[target.cli.version]\nuserdefined = \"\"\n", want: "target cli version"},
		{name: "unknown field", manifest: valid + "patch = 1\n", want: "unknown version field patch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errors := ParseManifestVersions(test.manifest)
			found := false
			for _, err := range errors {
				if strings.Contains(err.Error(), test.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("errors = %v, want one containing %q", errors, test.want)
			}
		})
	}
	if _, errors := ParseManifestVersions(valid); len(errors) != 0 {
		t.Fatalf("valid manifest errors = %v", errors)
	}
}
