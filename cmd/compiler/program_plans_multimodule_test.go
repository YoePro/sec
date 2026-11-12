package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/sema"
)

// The initialization and shutdown plans of a multi-module command do not
// depend on import order in the entry module or on the order in which source
// files are written to disk: statics of every module and the runtime
// requirements reachable through imported functions, including a task an
// imported module spawns from its own module-local function, are identical.
// Rules: rules/compiler/initialization.md — §§ 4, 16, 43.
func TestProgramPlansIgnoreImportAndFileOrder(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	modules := map[string]string{
		"alpha/alpha.sec": "module alpha\n\nlet Base: int := 40\n\nfn Get() int {\n    return Base\n}\n\nfn Work() void {\n}\n\nfn Start() Result[void, AllocationError] {\n    let job := try spawn task Work() { Err(_) => { return Ok() } }\n    await job\n    let mem := try Arena.WithCapacity(64u)\n    return Ok()\n}\n",
		"beta/beta.sec":   "module beta\n\nlet mut Count: int\nlet Scale: int := 2\n\nfn Value() int {\n    return Scale\n}\n",
	}
	render := func(imports []string, fileOrder []string) string {
		for _, name := range fileOrder {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(modules[name]), 0644); err != nil {
				t.Fatal(err)
			}
		}
		var source strings.Builder
		source.WriteString("module main\n\n")
		for _, path := range imports {
			source.WriteString("import \"" + path + "\"\n")
		}
		source.WriteString("\nlet Local: int := 1\n\nfn main() int {\n    match alpha.Start() {\n        Ok(_) => return alpha.Get() + beta.Value() - 42 - Local + 1\n        Err(error) => return 1\n    }\n}\n")
		mainFile := filepath.Join(dir, "main.sec")
		if err := os.WriteFile(mainFile, []byte(source.String()), 0644); err != nil {
			t.Fatal(err)
		}
		analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(source.String(), mainFile, hostCompilerTarget(), false)
		module, anchor := entryModuleOf(analyzed.Program, mainFile)
		initialization, shutdown, errors := analyzed.Analyzer.ProgramPlans(sema.ProgramTargetCommand, module, anchor)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		return initialization.String() + "\n" + sema.ProgramPlanFingerprint(initialization, shutdown)
	}
	want := render([]string{"alpha", "beta"}, []string{"alpha/alpha.sec", "beta/beta.sec"})
	for _, name := range []string{"static alpha.Base mutable=false init=40", "static beta.Count mutable=true init=", "static beta.Scale mutable=false init=2", "static main.Local", "runtime AllocatorRuntime", "runtime TaskScheduler"} {
		if !strings.Contains(want, name) {
			t.Fatalf("multi-module plan lacks %q:\n%s", name, want)
		}
	}
	if got := render([]string{"beta", "alpha"}, []string{"beta/beta.sec", "alpha/alpha.sec"}); got != want {
		t.Fatalf("import/file order changed the plan:\n%s\nvs\n%s", got, want)
	}
}
