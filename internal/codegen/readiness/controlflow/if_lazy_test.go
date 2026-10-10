package controlflow_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/codegen/llvm"
	legacy "sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/lexer"
	mlirtools "sec/internal/mlir"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestIfLazyEvaluationExecution checks observable condition/body traces rather
// than IR spelling: conditions execute once, logical RHS calls only execute on
// their selected edge, and else-if chains stop at their first matching branch.
// Rules: rules/control-flow/flowcontrol_if.md §§5–6;
// rules/foundations/operators.md — "Short-circuit evaluation";
// rules/corrections/applied/correction22-20260823.md — "Normative requirements".
func TestIfLazyEvaluationExecution(t *testing.T) {
	executeControlFlowFixture(t, "if_lazy", "20 lazy if scenarios passed\n")
}

// executeControlFlowFixture verifies and executes observable control-flow
// traces through the existing backends and both scalar plans at O0 and O2.
// Rules: rules/control-flow/flowcontrol_if.md §§5–6;
// rules/control-flow/flowcontrol_switch.md §§5–6, 12–15, 19.
func executeControlFlowFixture(t *testing.T, fixture, expected string) {
	t.Helper()

	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	fixtureDir, err := filepath.Abs(filepath.Join("../../../../testdata/control_flow", fixture))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(fixtureDir, "evaluation.sec")
	input, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{"llvm", "mlir"} {
		for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
			t.Run(backend+"/"+triple, func(t *testing.T) {
				plan, err := targetplan.Plan(triple)
				if err != nil {
					t.Fatal(err)
				}
				parsed := parser.New(lexer.NewWithFile(string(input), file)).Parse()
				if parsed.HasErrors {
					t.Fatal(parsed.Diagnostics)
				}
				analyzer := sema.NewAnalyzerWithScalarPlan(plan)
				if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
					t.Fatal(errors)
				}
				dir := t.TempDir()
				ir := filepath.Join(dir, "module.ll")
				if backend == "llvm" {
					output, err := llvm.GenerateAnalyzed(parsed.Program, analyzer, triple)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(ir, []byte(output), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					bin := os.Getenv("SEC_LEGACY_MLIR_BIN")
					toolchain := mlirtools.NewToolchain(bin)
					if _, err := os.Stat(filepath.Join(toolchain.BinDir, "mlir-opt")); err != nil {
						if bin != "" {
							t.Fatal(err)
						}
						t.Skip("legacy MLIR verifier unavailable")
					}
					output, err := legacy.GenerateAnalyzed(parsed.Program, analyzer, triple)
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(dir, "module.mlir")
					if err := os.WriteFile(path, []byte(output), 0600); err != nil {
						t.Fatal(err)
					}
					if err := toolchain.Verify(path); err != nil {
						t.Fatal(err)
					}
					if err := toolchain.TranslateToLLVMIR(path, ir); err != nil {
						t.Fatal(err)
					}
				}
				for _, optimization := range []string{"-O0", "-O2"} {
					object := filepath.Join(dir, "module"+optimization+".o")
					if output, err := exec.Command(clang, "-target", triple, optimization, "-c", ir, "-o", object).CombinedOutput(); err != nil {
						t.Fatalf("object compilation: %v\n%s", err, output)
					}
					if strings.HasPrefix(triple, "armv7") {
						continue // Cross-target object validation; execution uses the host target.
					}
					binary := filepath.Join(dir, "run"+optimization)
					if output, err := exec.Command(clang, optimization, object, filepath.Join(fixtureDir, "harness.c"), "-o", binary).CombinedOutput(); err != nil {
						t.Fatalf("link: %v\n%s", err, output)
					}
					output, err := exec.Command(binary).CombinedOutput()
					if err != nil || string(output) != expected {
						t.Fatalf("%s execution: %v\n%s", optimization, err, output)
					}
				}
			})
		}
	}
}
