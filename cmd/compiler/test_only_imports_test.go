package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestTreeProject builds a project with a production module, a
// command, a tests/ helper module and a tests/ integration test.
func writeTestTreeProject(t *testing.T, app string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".sec/sec.toml":             "[project]\nname='demo'\nuuid='550e8400-e29b-41d4-a716-446655440000'\n[version]\nmajor=1\nminor=0\nrevision=0\nbuild=0\nformat='[major].[minor].[revision]'\n[target.app]\nkind='command'\nsource='cmd/app'\n",
		"calc/calc.sec":             "module calc\n\nfn Add(a: int, b: int) int {\n    return a + b\n}\n",
		"tests/helpers/helpers.sec": "module helpers\n\nfn Two() int {\n    return 2\n}\n",
		"tests/calc/calc_test.sec":  "module calc\n\nimport \"calc\"\nimport \"tests/helpers\"\n\ntest \"adds\" {\n    testing.ExpectEqual(4, calc.Add(helpers.Two(), 2))\n}\n",
		"cmd/app/main.sec":          app,
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// testing.md §§ 8.1, 25.2, 26.1-26.2: tests/ modules participate only in the
// TestCompilationPlan of sec test, where Test -> Test and Test -> Production
// imports resolve; a production check never includes them, and production
// source importing one is rejected in both modes with S1145.
func TestProjectTestsTreeIsTestOnly(t *testing.T) {
	root := writeTestTreeProject(t, "module main\n\nimport \"calc\"\n\nfn main() int {\n    return calc.Add(1, 2)\n}\n")
	var out bytes.Buffer
	if status := runTestCommand([]string{"--list", root}, &out); status != testExitSuccess || !strings.Contains(out.String(), `calc "adds"`) {
		t.Fatalf("sec test --list = %d\n%s", status, out.String())
	}
	if _, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(root, "cmd", "app", "main.sec")); code != 0 {
		t.Fatalf("production check = %d %s", code, stderr)
	}
	if _, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(root, "tests", "calc", "calc_test.sec")); code != 3 || !strings.Contains(stderr, "S1145") || !strings.Contains(stderr, `test-only module "tests/helpers" participates only in a TestCompilationPlan`) {
		t.Fatalf("production check of a tests/ file = %d %s", code, stderr)
	}

	leaking := writeTestTreeProject(t, "module main\n\nimport \"tests/helpers\"\n\nfn main() int {\n    return helpers.Two()\n}\n")
	if _, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(leaking, "cmd", "app", "main.sec")); code != 3 || !strings.Contains(stderr, "S1145") {
		t.Fatalf("production import of tests/ = %d %s", code, stderr)
	}
	whiteBox := writeTestTreeProject(t, "module main\n\nfn main() int {\n    return 0\n}\n")
	if err := os.WriteFile(filepath.Join(whiteBox, "calc", "extra.sec"), []byte("module calc\n\nimport \"tests/helpers\"\n\nfn Three() int {\n    return helpers.Two() + 1\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(whiteBox, "calc", "calc_test.sec"), []byte("module calc\n\ntest \"three\" {\n    testing.ExpectEqual(3, Three())\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCLIForDiagnostics(t, "test", "--list", filepath.Join(whiteBox, "calc")); code == 0 || !strings.Contains(stderr, "S1145") {
		t.Fatalf("white-box production source importing tests/ = %d %s", code, stderr)
	}
}
