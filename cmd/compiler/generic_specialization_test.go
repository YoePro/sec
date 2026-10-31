package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genericSpecializationCLIFixture copies a fixture into a root with an empty
// sec/platform, so work-in-progress library sources do not take part.
func genericSpecializationCLIFixture(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/generics/specialization/" + name)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, name)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

// Methods of a generic implementation emit one verified specialization per
// receiver instance through Semantic IR and Sec MLIR.
// Rules: rules/compiler/monomorphization.md §§ 6, 9.3; package15-todo.md P15-47.
func TestGenericImplMethodSpecializationCLI(t *testing.T) {
	file := genericSpecializationCLIFixture(t, "impl_methods.sec")
	stdout, output, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", "linux-amd64", "-o", "-")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, want := range []string{`"main::Box.Get[T=int]()"`, `"main::Box.Get[T=bool]()"`, `"main::Box.Bump[T=int]()"`, "specialization(Box.Get: T="} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %s in:\n%s", want, stdout)
		}
	}
	if os.Getenv("SEC_MLIR_BIN") == "" {
		return
	}
	stdout, output, code = runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", "linux-amd64")
	if code != 0 || !strings.Contains(stdout, `sec.function_id = "main::Box.Bump[T=int]()"`) {
		t.Fatal(code, stdout, output)
	}
}
