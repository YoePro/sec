package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The compiler rejects a repeated import of one canonical module in a source
// file (also through an alias) with S1217 and a self-import with S1218,
// keeping the first import as related evidence.
// Rules: rules/projects/modules.md — §§ 14, 21.
func TestCLIRejectsDuplicateAndSelfImports(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "util"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "util", "util.sec"), []byte("module util\n\nfn One() int {\n    return 1\n}\n"), 0644); err != nil {
		t.Fatal(err)
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
