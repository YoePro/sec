package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/projects/projects.md — §§7(7),14,53(8); diagnostics.md — §9.
func TestValidateProjectRootModules(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/root_module/library.sec")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	manifest := filepath.Join(root, ".sec", "sec.toml")
	path := filepath.Join(root, "library.sec")
	program := parser.New(lexer.NewWithFile(string(source), path)).Parse().Program
	text := strings.Replace(string(fixture), `module = "suite"`, `module = "codec"`, 1)
	if errors := ValidateProjectRootModules(program, manifest, text); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, override := range []string{`module = "wrong"`, `module = 42`, `module = "codec.wrong"`, `module = '''codec'''`, `module = "codec"` + "\nmodule = \"wrong\""} {
		bad := strings.Replace(text, `module = "codec"`, override, 1)
		errors := ValidateProjectRootModules(program, manifest, bad)
		if len(errors) != 1 || errors[0].ID != diagnostics.ProjectRootModuleMismatch || errors[0].Line != 1 || errors[0].Column != 8 || errors[0].EndColumn != 13 || errors[0].File != path {
			t.Fatal(override, errors)
		}
	}
	// The root contract must not constrain declarations of nested modules.
	nested := parser.New(lexer.NewWithFile(string(source), filepath.Join(root, "nested", "library.sec"))).Parse().Program
	if errors := ValidateProjectRootModules(nested, manifest, strings.Replace(text, `module = "codec"`, `module = "wrong"`, 1)); len(errors) != 0 {
		t.Fatal(errors)
	}
	definition, ok := diagnostics.Lookup(diagnostics.ProjectRootModuleMismatch)
	if !ok || definition.Name != "projects.root-module-name-mismatch" || !definition.Mandatory || definition.DefaultSeverity != diagnostics.SeverityError {
		t.Fatal(definition)
	}
}
