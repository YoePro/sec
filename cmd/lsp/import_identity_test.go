package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// The editor reports the same canonical import-identity errors as the
// compiler: a repeated import of one module in a source file (under any
// alias), a module importing itself, and two imports binding one qualifier.
// Rules: rules/projects/modules.md — §§ 14, 21.
func TestAnalyzeReportsDuplicateAndSelfImports(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sec", "stdlib"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "app", "util"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app", "util", "util.sec"), []byte("module util\n\nfn One() int {\n    return 1\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"a/util", "b/util"} {
		if err := os.MkdirAll(filepath.Join(dir, "app", module), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "app", module, "util.sec"), []byte("module util\n\nfn One() int {\n    return 1\n}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sourcePath := filepath.Join(dir, "app", "main.sec")
	selfSource := "module util\n\nimport \"util\"\n"
	if err := os.WriteFile(filepath.Join(dir, "app", "util", "extra.sec"), []byte(selfSource), 0644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, source, code, message string
	}{
		{sourcePath, "module main\n\nimport \"util\"\nimport u \"util\"\n\nfn Use() int {\n    return u.One()\n}\n", diagnostics.DuplicateImport, `module "util" is already imported in this source file`},
		{sourcePath, "module main\n\nimport \"a/util\"\nimport \"b/util\"\n\nfn Use() int {\n    return util.One()\n}\n", diagnostics.ImportQualifierCollision, `import "b/util" binds qualifier util`},
		{filepath.Join(dir, "app", "util", "extra.sec"), selfSource, diagnostics.SelfImport, `module cannot import itself through "util"`},
	} {
		found := false
		for _, reported := range analyze(uriFromPath(test.path), test.source) {
			if reported.Code == test.code && strings.Contains(reported.Message, test.message) {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s %q in %+v", test.code, test.message, analyze(uriFromPath(test.path), test.source))
		}
	}
	valid := "module main\n\nimport \"util\"\n\nfn Use() int {\n    return util.One()\n}\n"
	if reported := analyze(uriFromPath(sourcePath), valid); len(reported) != 0 {
		t.Fatalf("single import produced diagnostics: %+v", reported)
	}
}
