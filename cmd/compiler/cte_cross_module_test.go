package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/sema"
)

// A required compile-time position executes a public function of an imported
// source module, which runs its module-private helper in its own origin
// scope; the importer still cannot name that helper.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 42–44; rules/projects/modules.md — visibility.
func TestCLICompileTimeEvaluationAcrossModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "util"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"util/util.sec": "module util\n\nfn _double(n: int) int {\n    return n * 2\n}\n\nfn Size() int {\n    return _double(50)\n}\n",
		"main.sec":      "module main\n\nimport \"util\"\n\ntype Small int range 0..util.Size()\n\nlet Limit: int := util.Size() + 1\n\nfn main() int {\n    let s: Small := 101\n    return Limit\n}\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	main := filepath.Join(dir, "main.sec")
	_, stderr, code := runCLIForDiagnostics(t, "check", main)
	if code == 0 || !strings.Contains(stderr, "value 101 violates range contract Small 0..100") || strings.Count(stderr, "error") > 2 {
		t.Fatalf("cross-module CTE: %d %s", code, stderr)
	}
	leaked := strings.Replace(files["main.sec"], "util.Size() + 1", "util._double(3)", 1)
	if err := os.WriteFile(main, []byte(leaked), 0644); err != nil {
		t.Fatal(err)
	}
	_, stderr, _ = runCLIForDiagnostics(t, "check", main)
	if !strings.Contains(stderr, "function util._double is not accessible from module main") {
		t.Fatalf("private helper leaked to the importer: %s", stderr)
	}
}

// A required evaluation that executes an imported module's function records
// that module's semantic artifact and the executed declarations among its
// dependencies, so a change to the imported module invalidates the value.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 58, 60(3).
func TestCompileTimeDependenciesNameImportedModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "util"), 0755); err != nil {
		t.Fatal(err)
	}
	util := "module util\n\nfn _double(n: int) int {\n    return n * 2\n}\n\nfn Size() int {\n    return _double(50)\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "util", "util.sec"), []byte(util), 0644); err != nil {
		t.Fatal(err)
	}
	source := "module main\n\nimport \"util\"\n\nlet Limit: int := util.Size() + 1\n\nfn main() int {\n    return Limit - 101\n}\n"
	mainFile := filepath.Join(dir, "main.sec")
	if err := os.WriteFile(mainFile, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(source, mainFile, hostCompilerTarget(), false)
	for _, static := range analyzed.Analyzer.ResolvedStaticDeclarations() {
		if static.Name != "Limit" {
			continue
		}
		dependencies, ok := analyzed.Analyzer.CompileTimeDependencies(static.Declaration.Value)
		imported, declarations := []string{}, 0
		for _, dependency := range dependencies {
			switch dependency.Kind {
			case sema.CTEDependsOnImportedModule:
				imported = append(imported, dependency.Identity)
			case sema.CTEDependsOnDeclaration:
				declarations++
			}
		}
		if !ok || len(imported) != 1 || imported[0] != "util" || declarations != 2 {
			t.Fatalf("dependencies = %v", dependencies)
		}
		return
	}
	t.Fatal("Limit not found")
}
