package secmlir

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// Enum failure paths must survive nominal erasure, checked arithmetic, Result
// transport and LLVM lowering on both scalar plans; execute success and error
// cases through the native ABI on the host plan.
// Rules: rules/declarations/enums.md §12.1; rules/compiler/semantic_ir.md §124.
func TestCheckedEnumConversionPipeline(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	const fixture = "../../../testdata/semantic_ir/enum_checked_conversion/"
	source, err := os.ReadFile(fixture + "native.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			plan := testPlan(width)
			p := parser.New(lexer.NewWithFile(string(source), fixture+"native.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(plan)
			if errors := analyzer.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(program, analyzer, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 23})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			output, err := Emit(module, plan)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			input := filepath.Join(dir, "enum.mlir")
			llvm := filepath.Join(dir, "llvm.mlir")
			if err := os.WriteFile(input, output, 0600); err != nil {
				t.Fatal(err)
			}
			if data, err := exec.Command(tool, input, "--sec-lower-to-llvm=entry=", "--verify-each", "-o", llvm).CombinedOutput(); err != nil {
				t.Fatalf("LLVM lowering: %v\n%s", err, data)
			}
			if width != 64 {
				return
			}
			t.Run("native", func(t *testing.T) {
				bin := os.Getenv("SEC_LEGACY_MLIR_BIN")
				if bin == "" || runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
					t.Skip("native requires linux-amd64 and SEC_LEGACY_MLIR_BIN")
				}
				clang, err := exec.LookPath("clang")
				if err != nil {
					t.Fatal(err)
				}
				ir := filepath.Join(dir, "enum.ll")
				binary := filepath.Join(dir, "enum-native")
				harness, err := filepath.Abs(fixture + "native.c")
				if err != nil {
					t.Fatal(err)
				}
				commands := [][]string{
					{filepath.Join(bin, "mlir-translate"), llvm, "--mlir-to-llvmir", "-o", ir},
					{clang, "-target", "x86_64-pc-linux-gnu", ir, harness, "-o", binary},
					{binary},
				}
				for _, command := range commands {
					if data, err := exec.Command(command[0], command[1:]...).CombinedOutput(); err != nil {
						t.Fatalf("%s: %v\n%s", command[0], err, data)
					}
				}
			})
		})
	}
}
