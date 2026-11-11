package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The compiler rejects a repeated import of one canonical module in a source
// file (also through an alias) with S1217 and a self-import with S1218,
// keeping the first import as related evidence; two imports binding one local
// qualifier (distinct modules sharing a ModuleName, or one alias reused)
// report S1220, while an alias resolves the collision.
// Rules: rules/projects/modules.md — §§ 14, 21.
func TestCLIRejectsDuplicateAndSelfImports(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "util"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "util", "util.sec"), []byte("module util\n\nfn One() int {\n    return 1\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"a/util", "b/util"} {
		if err := os.MkdirAll(filepath.Join(dir, module), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, module, "util.sec"), []byte("module util\n\nfn One() int {\n    return 1\n}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	main := filepath.Join(dir, "main.sec")
	cyc := filepath.Join(dir, "cycle", "cycle.sec")
	if err := os.MkdirAll(filepath.Dir(cyc), 0755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, source string
		want         []string
	}{
		{main, "module main\n\nimport \"util\"\nimport u \"util\"\n\nfn main() int {\n    return u.One()\n}\n",
			[]string{"S1217", `module "util" is already imported in this source file`, "first import of this module", "main.sec:3:8"}},
		{cyc, "module cycle\n\nimport \"cycle\"\n\nfn Value() int {\n    return 0\n}\n",
			[]string{"S1218", `module cannot import itself through "cycle"`}},
		{main, "module main\n\nimport \"a/util\"\nimport \"b/util\"\n\nfn main() int {\n    return util.One()\n}\n",
			[]string{"S1220", `import "b/util" binds qualifier util`, "earlier import binding util", "main.sec:3:8"}},
		{main, "module main\n\nimport x \"a/util\"\nimport x \"b/util\"\n\nfn main() int {\n    return x.One()\n}\n",
			[]string{"S1220", `import "b/util" binds qualifier x`}},
	} {
		if err := os.WriteFile(test.path, []byte(test.source), 0644); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runCLIForDiagnostics(t, "check", test.path)
		if code != 3 {
			t.Fatalf("exit code %d, stderr:\n%s", code, stderr)
		}
		for _, want := range test.want {
			if !strings.Contains(stderr, want) {
				t.Fatalf("missing %q in:\n%s", want, stderr)
			}
		}
	}
	if err := os.WriteFile(main, []byte("module main\n\nimport \"util\"\n\nfn main() int {\n    return util.One()\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCLIForDiagnostics(t, "check", main); code != 0 {
		t.Fatalf("single import failed (%d):\n%s", code, stderr)
	}
}

// A module below an internal directory is importable only from the directory
// that immediately contains that internal directory (or below it); a
// root-level internal directory is private to the whole project.
// Rules: rules/projects/modules.md — § 13, § 21.
func TestCLIEnforcesInternalImportAccess(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"services/internal/transport/transport.sec": "module transport\n\nfn Send() int {\n    return 1\n}\n",
		"internal/shared/shared.sec":                "module shared\n\nfn Value() int {\n    return 2\n}\n",
		"services/api/api.sec":                      "module api\n\nimport \"services/internal/transport\"\nimport \"internal/shared\"\n\nfn Call() int {\n    return transport.Send() + shared.Value()\n}\n",
		"other/other.sec":                           "module other\n\nimport \"services/internal/transport\"\n\nfn Call() int {\n    return transport.Send()\n}\n",
		".sec/sec.toml":                             "[project]\nname = \"internal-access\"\n[version]\nmajor = 0\nminor = 1\nrevision = 0\nbuild = 0\nformat = \"[major].[minor].[revision].[build]\"\n",
	}
	for path, source := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(dir, "services", "api", "api.sec")); code != 0 {
		t.Fatalf("allowed internal imports failed (%d):\n%s", code, stderr)
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(dir, "other", "other.sec"))
	if code == 0 || !strings.Contains(stderr, "S1149") || !strings.Contains(stderr, `internal modules are accessible only below "services"`) {
		t.Fatalf("internal import from outside was not rejected (%d):\n%s", code, stderr)
	}
}
