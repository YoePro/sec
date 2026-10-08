package readiness_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"sec/internal/codegen/llvm"
	"sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/lexer"
	mlirtools "sec/internal/mlir"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: types/types.md — Explicit conversions; memory/layout.md — scalar facts.
func TestLegacyRuntimeIntegerSignedExecution(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	source, err := os.ReadFile("../../../testdata/codegen/runtime_integer_conversions/signed_extension.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		for _, backend := range []string{"llvm", "mlir"} {
			t.Run(triple+"/"+backend, func(t *testing.T) {
				plan, err := targetplan.Plan(triple)
				if err != nil {
					t.Fatal(err)
				}
				parsed := parser.New(lexer.New(string(source))).Parse()
				if parsed.HasErrors {
					t.Fatal(parsed.Diagnostics)
				}
				analyzer := sema.NewAnalyzerWithScalarPlan(plan)
				if errors := analyzer.Analyze(parsed.Program); len(errors) > 0 {
					t.Fatal(errors)
				}
				dir := t.TempDir()
				llvmFile := filepath.Join(dir, "module.ll")
				if backend == "llvm" {
					output, err := llvm.GenerateAnalyzed(parsed.Program, analyzer, triple)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(llvmFile, []byte(output), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					bin := os.Getenv("SEC_LEGACY_MLIR_BIN")
					if bin == "" {
						home, err := os.UserHomeDir()
						if err != nil {
							t.Fatal(err)
						}
						bin = filepath.Join(home, "mlir", "llvm-project", "build", "bin")
					}
					tool := filepath.Join(bin, "mlir-opt")
					if _, err := os.Stat(tool); err != nil {
						if os.Getenv("SEC_LEGACY_MLIR_BIN") != "" {
							t.Fatal(err)
						}
						t.Skip("legacy MLIR verifier unavailable")
					}
					output, err := mlir.GenerateAnalyzed(parsed.Program, analyzer, triple)
					if err != nil {
						t.Fatal(err)
					}
					mlirFile := filepath.Join(dir, "module.mlir")
					if err := os.WriteFile(mlirFile, []byte(output), 0600); err != nil {
						t.Fatal(err)
					}
					if diagnostics, err := exec.Command(tool, mlirFile, "-o", os.DevNull).CombinedOutput(); err != nil {
						t.Fatal(err, string(diagnostics))
					}
					if err := mlirtools.NewToolchain(bin).TranslateToLLVMIR(mlirFile, llvmFile); err != nil {
						t.Fatal(err)
					}
				}
				binary := filepath.Join(dir, "program")
				args := []string{"-x", "ir", llvmFile, "-target", triple, "-o", binary}
				if plan.PointerWidthBits == 32 {
					args = append(args, "-c")
				}
				if diagnostics, err := exec.Command(clang, args...).CombinedOutput(); err != nil {
					t.Fatal(err, string(diagnostics))
				}
				if plan.PointerWidthBits == 64 {
					if diagnostics, err := exec.Command(binary).CombinedOutput(); err != nil {
						t.Fatal("signedness lost during execution", err, string(diagnostics))
					}
				}
			})
		}
	}
}
