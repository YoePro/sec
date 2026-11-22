package sema

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// testing.md §§ 3.1, 8.1, 25.2: the mode is a named value defaulting to
// production; tests/ and *_test.sec are test-only sources; a tests/ import is
// rejected in production plans and from production source in test plans,
// and allowed from test-only source in a TestCompilationPlan.
func TestCompilationModeAndTestOnlyImports(t *testing.T) {
	if mode := NewAnalyzer().CompilationMode(); mode != CompilationProduction {
		t.Fatalf("default mode = %q", mode)
	}
	a := NewAnalyzer()
	a.SetCompilationMode(CompilationTest)
	if a.CompilationMode() != CompilationTest {
		t.Fatal("mode not retained")
	}
	root := t.TempDir()
	for _, name := range []string{".sec/sec.toml", "tests/helpers/helpers.sec", "calc/calc.sec", "calc/calc_test.sec", "tests/calc/calc_test.sec"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("module x\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	at := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
	for name, want := range map[string]bool{"calc/calc.sec": false, "calc/calc_test.sec": true, "tests/helpers/helpers.sec": true, "tests/calc/calc_test.sec": true} {
		if IsTestOnlySource(at(name)) != want {
			t.Fatalf("IsTestOnlySource(%s) != %v", name, want)
		}
	}
	if IsTestOnlySource(filepath.Join(t.TempDir(), "tests", "loose.sec")) {
		t.Fatal("tests/ outside a project is not a test tree")
	}
	helpers := []string{at("tests/helpers/helpers.sec")}
	importFrom := func(file string) *ast.ImportStatement {
		return &ast.ImportStatement{Token: lexer.Token{File: file, Line: 3, Column: 1}, Path: "tests/helpers"}
	}
	for _, test := range []struct {
		importer string
		mode     CompilationMode
		fails    bool
	}{
		{"calc/calc.sec", CompilationProduction, true},
		{"tests/calc/calc_test.sec", CompilationProduction, true},
		{"calc/calc.sec", CompilationTest, true},
		{"calc/calc_test.sec", CompilationTest, false},
		{"tests/calc/calc_test.sec", CompilationTest, false},
	} {
		issues := TestOnlyImportIssues(importFrom(at(test.importer)), "", helpers, test.mode)
		if test.fails != (len(issues) == 1 && issues[0].ID == diagnostics.ProductionDependsOnTest) || !test.fails && len(issues) != 0 {
			t.Fatalf("%s in %s: %v", test.importer, test.mode, issues)
		}
	}
	if issues := TestOnlyImportIssues(importFrom(at("calc/calc.sec")), "", []string{at("calc/calc.sec")}, CompilationProduction); len(issues) != 0 {
		t.Fatalf("production import rejected: %v", issues)
	}
}
