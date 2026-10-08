package scalar_test

import (
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	mlir "sec/internal/codegen/mlir"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	mlirtoolchain "sec/internal/mlir"
	"sec/internal/parser"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// TestLegacyMLIRTargetIntegers pins canonical native carriers and fixed-width
// controls across scalar and aggregate emission; the real verifier checks the
// complete emitted LLVM-dialect module when configured.
// Rules: rules/types/types.md — "int and uint", "Context shaping";
// rules/declarations/enums.md — "Underlying type"; correction5.md — target facts.
func TestLegacyMLIRTargetIntegers(t *testing.T) {
	read := func(path string) string {
		data, err := os.ReadFile("../../../../testdata/codegen/" + path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	source := read("llvm_target_integers/scalars.sec") + read("mlir_target_integers/aggregates.sec")
	bounds := read("llvm_target_integers/bounds.sec.in")
	overflow := read("mlir_target_integers/native_32_overflow.sec")
	for _, arch := range []string{"armv7", "amd64"} {
		t.Run(arch, func(t *testing.T) {
			definition, _ := platformtarget.Find(platformtarget.Target{OS: "linux", Arch: arch})
			plan, err := definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			limit := new(big.Int).Lsh(big.NewInt(1), uint(plan.PointerWidthBits-1))
			maximum := new(big.Int).Sub(new(big.Int).Set(limit), big.NewInt(1)).String()
			minimum := new(big.Int).Neg(new(big.Int).Set(limit)).String()
			unsignedMax := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(plan.PointerWidthBits)), big.NewInt(1)).String()
			boundSource := strings.NewReplacer("@SIGNED_MAX@", maximum, "@SIGNED_MIN@", minimum, "@UNSIGNED_MAX@", unsignedMax).Replace(bounds)
			parsed := parser.New(lexer.New(source + boundSource)).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(plan)
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			output, err := mlir.GenerateAnalyzed(parsed.Program, analyzer, plan.LLVMTriple)
			if err != nil {
				t.Fatal(err)
			}
			native := "i32"
			if arch == "amd64" {
				native = "i64"
			}
			for _, want := range []string{
				"llvm.func @SignedIdentity(%value: " + native + ") -> " + native,
				"llvm.func @UnsignedIdentity(%value: " + native + ") -> " + native,
				"llvm.func @EnumIdentity(%value: " + native + ") -> " + native,
				"llvm.func @DefaultEnumIdentity(%value: " + native + ") -> " + native,
				"!llvm.struct<(" + native + ", " + native + ")>", "!llvm.array<2 x " + native + ">",
				"llvm.func @Length(%value.ptr: !llvm.ptr, %value.len: i64) -> " + native,
				"llvm.mlir.constant(" + maximum + " : " + native + ")",
				"llvm.mlir.constant(" + minimum + " : " + native + ")",
				"llvm.mlir.constant(" + unsignedMax + " : " + native + ")",
				"llvm.mlir.constant(-9223372036854775808 : i64)",
				"llvm.mlir.constant(18446744073709551615 : i64)",
				"llvm.func @Fixed(%value: i32) -> i32",
				"llvm.func @FixedUnsigned(%value: i64) -> i64",
			} {
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q:\n%s", want, output)
				}
			}
			if arch == "armv7" {
				for _, want := range []string{"llvm.zext", "llvm.sext"} {
					if !strings.Contains(output, want) {
						t.Fatalf("extension signedness lost: %s", output)
					}
				}
			}
			raw, err := mlir.GenerateWithTriple(parsed.Program, plan.LLVMTriple)
			if err != nil || raw != output {
				t.Fatal("entry points disagree", err)
			}
			other, _ := platformtarget.Find(platformtarget.Target{OS: "linux", Arch: map[string]string{"armv7": "amd64", "amd64": "armv7"}[arch]})
			if partial, err := mlir.GenerateAnalyzed(parsed.Program, analyzer, other.LLVMTriple); err == nil || partial != "" {
				t.Fatal("mismatched Sema target accepted")
			}
			p := parser.New(lexer.New(overflow)).Parse()
			failures := sema.NewAnalyzerWithScalarPlan(plan).Analyze(p.Program)
			if (len(failures) > 0) != (arch == "armv7") {
				t.Fatal("unexpected frontend bounds", failures)
			}
			partial, err := mlir.GenerateWithTriple(p.Program, plan.LLVMTriple)
			if arch == "armv7" {
				var unsupported *semantic.UnsupportedFeatureError
				if partial != "" || !errors.As(err, &unsupported) {
					t.Fatal("native literal silently widened", partial, err)
				}
			} else if err != nil || !strings.Contains(partial, "3000000000 : i64") {
				t.Fatal(partial, err)
			}
			t.Run("verify", func(t *testing.T) {
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
					t.Skip("legacy MLIR toolchain unavailable")
				}
				file := filepath.Join(t.TempDir(), "module.mlir")
				if err := os.WriteFile(file, []byte(output), 0600); err != nil {
					t.Fatal(err)
				}
				if diagnostics, err := exec.Command(tool, file, "-o", os.DevNull).CombinedOutput(); err != nil {
					t.Fatalf("MLIR verifier rejected module: %v\n%s\n%s", err, diagnostics, output)
				}
				llvmFile := file + ".ll"
				if err := mlirtoolchain.NewToolchain(bin).TranslateToLLVMIR(file, llvmFile); err != nil {
					t.Fatal(err)
				}
				if clang, err := exec.LookPath("clang"); err == nil {
					if diagnostics, err := exec.Command(clang, "-x", "ir", "-c", llvmFile, "-target", plan.LLVMTriple, "-o", llvmFile+".o").CombinedOutput(); err != nil {
						t.Fatalf("LLVM rejected translated target module: %v\n%s", err, diagnostics)
					}
				}

			})
		})
	}
	parsed := parser.New(lexer.New(read("llvm_target_integers/scalars.sec"))).Parse()
	if partial, err := mlir.GenerateWithTriple(parsed.Program, "unregistered-64-bit-target"); err == nil || partial != "" {
		t.Fatal("unknown target guessed a width")
	}
	reused := &mlir.Generator{}
	first, err := reused.Generate(parsed.Program)
	if err != nil {
		t.Fatal(err)
	}
	invalid := parser.New(lexer.New(read("mlir_target_integers/native_overflow_invalid.sec"))).Parse()
	if invalid.HasErrors {
		t.Fatal(invalid.Diagnostics)
	}
	if failures := sema.NewAnalyzer().Analyze(invalid.Program); len(failures) == 0 {
		t.Fatal("invalid unsigned fixture accepted")
	}
	// A failure must not retain constants, buffers or counters from an invocation.
	if partial, err := reused.Generate(invalid.Program); err == nil || partial != "" {
		t.Fatal("invalid invocation published output")
	}
	second, err := reused.Generate(parsed.Program)
	if err != nil || first != second {
		t.Fatal("generator retained failed invocation state", err)
	}
}
