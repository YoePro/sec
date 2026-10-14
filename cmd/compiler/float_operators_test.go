package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// TestBuildLegacyFloatOperators exercises parsing, Sema, LLVM-dialect MLIR
// verification, LLVM translation, native linking (including libm), and execution.
// Rules: rules/foundations/operators.md — Floating equality, ordering, remainder;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestBuildLegacyFloatOperators(t *testing.T) {
	binDir := os.Getenv("SEC_LEGACY_MLIR_BIN")
	if binDir == "" {
		t.Skip("SEC_LEGACY_MLIR_BIN is not set")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("native execution requires linux-amd64")
	}
	if !filepath.IsAbs(binDir) {
		t.Fatal("SEC_LEGACY_MLIR_BIN must be absolute")
	}
	for _, tool := range []string{"mlir-opt", "mlir-translate"} {
		if _, err := os.Stat(filepath.Join(binDir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"matrix", "effects"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			compiler := clang
			if name == "effects" {
				// Supply the foreign probes without putting tool paths into a script.
				compiler = filepath.Join(dir, "clang-with-probes")
				if err := os.WriteFile(compiler, []byte("#!/bin/sh\nexec \"$SEC_TEST_CLANG\" \"$@\" \"$SEC_TEST_EFFECTS_C\"\n"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("SEC_TEST_CLANG", clang)
				t.Setenv("SEC_TEST_EFFECTS_C", filepath.Join(repo, "testdata/codegen/float_operators/effects.c"))
			}
			binary := filepath.Join(dir, "float-operators")
			input := filepath.Join(repo, "testdata/codegen/float_operators", name+".sec")
			output, err := runBuildCLIInDirectory(t, repo, input, "--pipeline", "mlir", "--mlir-bin", binDir, "--clang", compiler, "--target", "linux-amd64", "-o", binary)
			if err != nil {
				t.Fatalf("build: %v\n%s", err, output)
			}
			if output, err := exec.Command(binary).CombinedOutput(); err != nil {
				t.Fatalf("execution: %v\n%s", err, output)
			}
		})
	}
}

func TestLegacyMLIRMathRuntimeUsesSelectedTarget(t *testing.T) {
	text := "    %result = llvm.frem %left, %right : f64\n"
	for _, target := range []string{"linux", "freebsd", "macos", "windows"} {
		var want []string
		if target == "linux" || target == "freebsd" {
			want = []string{"-lm"}
		}
		if got := legacyMLIRRuntimeLibraries(text, target); !reflect.DeepEqual(got, want) {
			t.Errorf("target %s: %v, want %v", target, got, want)
		}
	}
	for _, text := range []string{"", `llvm.mlir.global @text("llvm.frem")`, "    %r = llvm.srem %a, %b : i64"} {
		if got := legacyMLIRRuntimeLibraries(text, "linux"); len(got) != 0 {
			t.Errorf("unneeded libm dependency for %q: %v", text, got)
		}
	}
}
