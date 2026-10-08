package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	semantic "sec/internal/ir/semantic"
)

func TestParseEmitIRCommandArgs(t *testing.T) {
	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	input, output, gotTarget, ok := parseEmitIRCommandArgs([]string{"sample.sec", "-o", "sample.sir", "--target", "linux-amd64"}, target)
	if !ok || input != "sample.sec" || output != "sample.sir" || gotTarget != target {
		t.Fatalf("got %q %q %#v %t", input, output, gotTarget, ok)
	}
	_, output, _, ok = parseEmitIRCommandArgs([]string{"sample.sec"}, target)
	if !ok || output != "-" {
		t.Fatalf("default output = %q, ok=%t", output, ok)
	}
}

func TestFrontendRetainsAnalyzerForSemanticIR(t *testing.T) {
	source := "module main\nfn Answer() int { return 42 }\n"
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(source, "sample.sec", hostCompilerTarget(), false)
	if analyzed.Program == nil || analyzed.Analyzer == nil {
		t.Fatal("frontend discarded program or analyzer")
	}
	module, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	if text := semantic.Format(module); !strings.HasPrefix(text, "semantic-ir 1\n") {
		t.Fatalf("unexpected output:\n%s", text)
	}
}

// TestLegacyMLIRTargetScalarCLI retains frontend target facts through emission
// and preserves existing output files when a native literal overflows on 32 bits.
// Rules: rules/types/types.md — "int and uint"; correction5.md — backend widths;
// rules/compiler/compiler_pipeline.md — §61(3).
func TestLegacyMLIRTargetScalarCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile("../../testdata/codegen/llvm_target_integers/scalars.sec")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "main.sec")
	if err := os.WriteFile(file, input, 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"linux-armv7", "linux-amd64"} {
		stdout, diagnostics, code := runCLIForDiagnostics(t, "emit-mlir", file, "--target", target, "-o", "-")
		native := "i32"
		if target == "linux-amd64" {
			native = "i64"
		}
		if code != 0 || !strings.Contains(stdout, "llvm.func @SignedIdentity(%value: "+native+") -> "+native) || !strings.Contains(stdout, "llvm.func @UnsignedIdentity(%value: "+native+") -> "+native) {
			t.Fatal(target, code, stdout, diagnostics)
		}
	}
	overflow, err := os.ReadFile("../../testdata/codegen/mlir_target_integers/native_32_overflow.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, overflow, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "existing.mlir")
	if err := os.WriteFile(output, []byte("existing output"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, diagnostics, code := runCLIForDiagnostics(t, "emit-mlir", file, "--target", "linux-armv7", "-o", output, "--diagnostic-format=json")
	if code != 3 || stdout != "" {
		t.Fatal("overflow was not rejected by frontend", code, stdout, diagnostics)
	}
	unchanged, err := os.ReadFile(output)
	if err != nil || string(unchanged) != "existing output" {
		t.Fatal("failed emission damaged output", string(unchanged), err)
	}
	stdout, diagnostics, code = runCLIForDiagnostics(t, "emit-mlir", file, "--target", "linux-amd64", "-o", "-")
	if code != 0 || !strings.Contains(stdout, "3000000000 : i64") {
		t.Fatal(code, stdout, diagnostics)
	}
}

// Rules: rules/types/types.md — Binary floating-point types; MD-014 correction §6.
func TestLegacyMLIRPlatformFloatCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile("../../testdata/codegen/llvm_platform_float/signature.sec")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "main.sec")
	if err := os.WriteFile(file, input, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "existing.mlir")
	if err := os.WriteFile(output, []byte("existing output"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, diagnostics, code := runCLIForDiagnostics(t, "emit-mlir", file, "--target", "linux-armv7", "-o", output, "--diagnostic-format=json")
	if code == 0 || stdout != "" || !strings.Contains(diagnostics, "float") || !strings.Contains(diagnostics, "unsupported") {
		t.Fatal(code, stdout, diagnostics)
	}
	contents, err := os.ReadFile(output)
	if err != nil || string(contents) != "existing output" {
		t.Fatal("rejected emission changed output", string(contents), err)
	}
	stdout, diagnostics, code = runCLIForDiagnostics(t, "emit-mlir", file, "--target", "linux-amd64", "-o", "-")
	if code != 0 || !strings.Contains(stdout, "@Identity(%value: f64) -> f64") {
		t.Fatal(code, stdout, diagnostics)
	}
}

// Rules: types/types.md — Explicit conversions; MD-012 correction §4.
func TestLegacyRuntimeIntegerConversionCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/codegen/runtime_integer_conversions/native_narrowing.sec")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "main.sec")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"emit-llvm", "emit-mlir"} {
		t.Run(command, func(t *testing.T) {
			output := filepath.Join(root, command+".out")
			if err := os.WriteFile(output, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, diagnostics, code := runCLIForDiagnostics(t, command, file, "--target", "linux-armv7", "-o", output, "--diagnostic-format=json")
			if code == 0 || stdout != "" || !strings.Contains(diagnostics, "MD-012") {
				t.Fatal(code, stdout, diagnostics)
			}
			contents, err := os.ReadFile(output)
			if err != nil || string(contents) != "existing output" {
				t.Fatal(string(contents), err)
			}
			stdout, diagnostics, code = runCLIForDiagnostics(t, command, file, "--target", "linux-amd64", "-o", "-")
			if code != 0 || !strings.Contains(stdout, "Convert") {
				t.Fatal(code, stdout, diagnostics)
			}
		})
	}
}
