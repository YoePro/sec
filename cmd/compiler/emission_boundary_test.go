package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mlirtoolchain "sec/internal/mlir"
)

// TestMLIREmissionBoundaries checks CLI boundary/target labeling in files and
// stdout without requiring installed backend tools. The stub exercises only
// verifier transport; real LLVM-dialect parsing is tested separately below.
// Rules: rules/compiler/compiler_pipeline.md — §§59(1–3),60(1–3).
func TestMLIREmissionBoundaries(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/emission_boundary.sec")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "main.sec")
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(root, "tools")
	if err := os.MkdirAll(tools, 0755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"mlir-opt", "sec-mlir-opt"} {
		if err := os.WriteFile(filepath.Join(tools, tool), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range []string{"emit-mlir", "emit-sec-mlir"} {
		for _, target := range []string{"linux-amd64", "linux-armv7"} {
			args := []string{command, input, "--target", target, "--mlir-bin", tools, "-o"}
			boundary := "Sec dialect MLIR"
			if command == "emit-mlir" {
				boundary = "LLVM dialect MLIR"
			}
			stdout, diagnostics, code := runCLIForDiagnostics(t, append(args, "-")...)
			prefix := "// Sec compiler MLIR boundary: " + boundary + "\n// Selected target: " + target + "\n"
			if code != 0 || !strings.HasPrefix(stdout, prefix) {
				t.Fatalf("%s/%s: %d\n%s\n%s", command, target, code, stdout, diagnostics)
			}
			if command == "emit-sec-mlir" && !strings.Contains(stdout, "sec.dialect_version =") {
				t.Fatal("lost Sec schema identity")
			}
			output := filepath.Join(root, command+".mlir")
			fileStdout, diagnostics, code := runCLIForDiagnostics(t, append(args, output)...)
			written, err := os.ReadFile(output)
			if code != 0 || err != nil || fileStdout != "" || string(written) != stdout {
				t.Fatalf("file/stdout differ: %d %v\n%s", code, err, diagnostics)
			}
		}
	}
	for _, tool := range []string{"mlir-opt", "sec-mlir-opt"} {
		if err := os.WriteFile(filepath.Join(tools, tool), []byte("#!/bin/sh\necho rejected >&2\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range []string{"emit-mlir", "emit-sec-mlir"} {
		for _, output := range []string{"-", filepath.Join(root, "existing.mlir")} {
			if output != "-" {
				if err := os.WriteFile(output, []byte("existing output"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{command, input, "-o", output, "--mlir-bin", tools}
			stdout, diagnostics, code := runCLIForDiagnostics(t, args...)
			if code != 4 || stdout != "" || !strings.Contains(diagnostics, "rejected") {
				t.Fatalf("failed verifier published output: %s %d %s %s", command, code, stdout, diagnostics)
			}
			if output != "-" {
				written, err := os.ReadFile(output)
				if err != nil || string(written) != "existing output" {
					t.Fatal("failed verifier replaced destination", err, string(written))
				}
			}
		}
	}
}

// TestLabeledLLVMMLIRParsesAndTranslates verifies that the LLVM dialect the
// single track produces keeps its labels parseable and both selected targets
// survive translation (MD-101).
// Rules: rules/compiler/compiler_pipeline.md — §60(1–3),§61(2).
func TestLabeledLLVMMLIRParsesAndTranslates(t *testing.T) {
	toolchain := mlirtoolchain.NewToolchain("")
	if _, err := os.Stat(filepath.Join(toolchain.BinDir, "sec-mlir-opt")); err != nil {
		t.Skip("Sec MLIR tools unavailable")
	}
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
			t.Fatal(err)
		}
		fixture, err := os.ReadFile("../../testdata/tooling/emission_boundary.sec")
		if err != nil {
			t.Fatal(err)
		}
		input := filepath.Join(root, "main.sec")
		if err := os.WriteFile(input, fixture, 0600); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(root, "labeled.mlir")
		stdout, diagnostics, code := runCLIForDiagnostics(t, "emit-mlir", input, "--target", target, "-o", output, "--mlir-bin", toolchain.BinDir)
		if code != 0 || stdout != "" {
			t.Fatalf("%s: %d %s %s", target, code, stdout, diagnostics)
		}
		llvmOutput := filepath.Join(root, "labeled.ll")
		if err := toolchain.TranslateToLLVMIR(output, llvmOutput); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(llvmOutput)
		definition, _ := findTargetDefinition(CompilerTarget{OS: "linux", Arch: strings.TrimPrefix(target, "linux-")})
		if err != nil || !strings.Contains(string(data), definition.LLVMTriple) {
			t.Fatalf("target lost: %v\n%s", err, data)
		}
	}
}

// TestMLIRUsageNamesBoundaries keeps command help explicit about both levels.
// Rules: rules/compiler/compiler_pipeline.md — §60(2).
func TestMLIRUsageNamesBoundaries(t *testing.T) {
	_, help, code := runCLIForDiagnostics(t, "emit-mlir")
	if code != 1 || !strings.Contains(help, "emit-sec-mlir = verified Sec dialect; emit-mlir = LLVM dialect") {
		t.Fatal(code, help)
	}
}

// TestLabeledSecMLIRParses verifies boundary comments with the actual Sec
// dialect driver, retaining mandatory verification before CLI publication.
// Rules: rules/compiler/compiler_pipeline.md — §59(1–3),§60(2).
func TestLabeledSecMLIRParses(t *testing.T) {
	bin := os.Getenv("SEC_MLIR_BIN")
	configured := bin != ""
	if !configured {
		var err error
		bin, err = filepath.Abs("../../build/sec-mlir/bin")
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(bin, "sec-mlir-opt")); err != nil {
		if configured {
			t.Fatal(err)
		}
		t.Skip("Sec MLIR verifier unavailable")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../testdata/tooling/emission_boundary.sec")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "main.sec")
	if err := os.WriteFile(input, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		stdout, diagnostics, code := runCLIForDiagnostics(t, "emit-sec-mlir", input, "--target", target, "-o", "-", "--mlir-bin", bin)
		if code != 0 || !strings.HasPrefix(stdout, "// Sec compiler MLIR boundary: Sec dialect MLIR\n// Selected target: "+target+"\n") {
			t.Fatalf("%s: %d %s %s", target, code, stdout, diagnostics)
		}
	}
}
