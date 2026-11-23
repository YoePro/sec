package project

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestTargetSourcesRespectTestTree checks the complete source-selection
// boundary independently of host path separators and without filesystem I/O.
// Rules: rules/tooling/testing.md — §8.1;
// rules/projects/projects.md — §§17–18,48.
func TestTargetSourcesRespectTestTree(t *testing.T) {
	for _, kind := range []string{"command", "library", "firmware", "test", ""} {
		for _, source := range []string{"tests", "tests/product", "./tests/product", "src/../tests/product", "tests/../cmd/product", "src/tests/product", "tests_extra/product", "cmd/product"} {
			text := "[target.product]\nkind = '" + kind + "'\n  source = '" + source + "' # selected entry\n"
			issues := ValidateTargetSourcesSnapshot(text)
			want := kind != "test" && (source == "tests" || source == "tests/product" || source == "./tests/product" || source == "src/../tests/product")
			if (len(issues) == 1) != want {
				t.Fatal(kind, source, issues)
			}
			if want && (issues[0].Line != 3 || issues[0].Column != 3 || issues[0].EndColumn <= issues[0].Column || !strings.Contains(issues[0].Message, "target product")) {
				t.Fatal(issues)
			}
		}
	}
	// Dotted/inline/quoted keys resolve through TOML structure, not text scans.
	text := "# source = 'tests/decoy'\r\n[parameters]\r\ndecoy = '''\r\n[target.fake]\r\nsource = 'tests/decoy'\r\n'''\r\n[target.\"product\"]\r\nkind = 'command'\r\n  source = 'tests/product'\r\n"
	issues := ValidateTargetSourcesSnapshot(text)
	if len(issues) != 1 || issues[0].Line != 9 || issues[0].Column != 3 {
		t.Fatal(issues)
	}
	inline := "target.product = { kind = 'library', source = 'tests/product' }"
	if issues := ValidateTargetSourcesSnapshot(inline); len(issues) != 1 || issues[0].Line != 1 {
		t.Fatal(issues)
	}
	if issues := ValidateTargetSourcesSnapshot("target = { broken"); len(issues) != 0 {
		t.Fatal(issues)
	}
}

// TestManifestRejectsProductionTestTarget prevents consumers of resolved
// manifests from bypassing the same snapshot rule enforced by CLI and LSP.
// Rules: rules/tooling/testing.md — §§8.1,26.5;
// rules/projects/projects.md — §§17–18,49.
func TestManifestRejectsProductionTestTarget(t *testing.T) {
	text := strings.Replace(manifestFixture(t), `source = "cmd/sec"`, `source = "tests/product"`, 1)
	manifest, issues := ParseManifest(filepath.Join(t.TempDir(), ".sec", "sec.toml"), text)
	if manifest != nil || len(issues) != 1 || !strings.Contains(issues[0].Message, "test-only source") {
		t.Fatal(manifest, issues)
	}
	text = strings.Replace(text, `kind = "command"`, `kind = "test"`, 1)
	if manifest, issues := ParseManifest(filepath.Join(t.TempDir(), ".sec", "sec.toml"), text); manifest == nil || len(issues) != 0 {
		t.Fatal(manifest, issues)
	}
}
