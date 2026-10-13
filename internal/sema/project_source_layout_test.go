package sema

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/projects/projects.md — §§15,48(2); diagnostics.md — §9.
func TestProjectSourceLayoutDiagnostics(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, ".sec", "sec.toml")
	text, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"cmd", "bin/store", ".sec/state", "services/internal"} {
		path := filepath.Join(root, directory, "main.sec")
		program := parser.New(lexer.NewWithFile(string(text), path)).Parse().Program
		issues := ValidateProjectSourceLayout(program, manifest)
		if len(issues) != 1 || issues[0].ID != "S1149" || issues[0].File != path || issues[0].Line != 1 || issues[0].Column != 8 || issues[0].EndColumn != 12 {
			t.Fatal(directory, issues)
		}
	}
	definition, found := diagnostics.Lookup(diagnostics.ProjectSourceLayout)
	if !found || !definition.Mandatory || definition.Name != "projects.invalid-source-layout" {
		t.Fatal(definition, found)
	}
}
