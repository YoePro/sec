package main

import (
	"os"
	"path/filepath"
	"testing"
)

// testing.md §§ 8.1, 25.2: the editor reports a production import of a
// project tests/ module with S1145 and resolves the same import from a
// tests/ integration test, which it analyzes in its test view.
func TestLSPProjectTestsTreeImports(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".sec/sec.toml":             "[project]\nname='demo'\nuuid='550e8400-e29b-41d4-a716-446655440000'\n[version]\nmajor=1\nminor=0\nrevision=0\nbuild=0\nformat='[major].[minor].[revision]'\n",
		"calc/calc.sec":             "module calc\n\nimport \"tests/helpers\"\n\nfn Three() int {\n    return helpers.Two() + 1\n}\n",
		"tests/helpers/helpers.sec": "module helpers\n\nfn Two() int {\n    return 2\n}\n",
		"tests/calc/calc_test.sec":  "module calc\n\nimport \"tests/helpers\"\n\ntest \"two\" {\n    testing.ExpectEqual(2, helpers.Two())\n}\n",
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
	production := filepath.Join(root, "calc", "calc.sec")
	items := analyze(uriFromPath(production), files["calc/calc.sec"])
	found := false
	for _, item := range items {
		found = found || item.Code == "S1145"
	}
	if !found {
		t.Fatalf("production import diagnostics = %+v", items)
	}
	integration := filepath.Join(root, "tests", "calc", "calc_test.sec")
	for _, item := range analyze(uriFromPath(integration), files["tests/calc/calc_test.sec"]) {
		if item.Code == "S1145" || item.Code == "S1051" {
			t.Fatalf("integration test import rejected: %+v", item)
		}
	}
}
