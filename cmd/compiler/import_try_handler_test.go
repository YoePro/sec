package main

import (
	"os"
	"path/filepath"
	"testing"
)

// An imported module's try handlers are module code: their bodies may name
// the module's own types and functions, which import qualification must
// rewrite like any other expression (rules/projects/modules.md — module
// identity of imported declarations).
func TestCLIImportedTryHandlersResolveModuleNames(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.sec": `module main

import "demo"

fn main() int {
    match demo.Narrow(300) {
        Ok(_) => return 0
        Err(_) => return 1
    }
}
`,
		"sec/stdlib/demo/demo.sec": `module demo

enum DemoError error {
    TooLarge,
}

fn _fallback() int8 {
    return 0
}

fn Narrow(value: int) Result[int8, DemoError] {
    let narrowed := try int8(value) {
        Err(_) => return Err(DemoError.TooLarge)
    }
    let other := try int8(value + 1) {
        Err(_) => _fallback()
    }
    discard other
    return Ok(narrowed)
}
`,
	}
	for relative, contents := range files {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, diagnostics, code := runCLIForDiagnostics(t, "sema", filepath.Join(root, "main.sec"))
	if code != 0 {
		t.Fatalf("imported try handlers must resolve their module's names:\n%s", diagnostics)
	}
}
