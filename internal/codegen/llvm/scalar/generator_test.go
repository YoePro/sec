package scalar_test

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	llvm "sec/internal/codegen/llvm"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// TestLegacyLLVMTargetIntegers independently verifies target-sized signatures,
// literals, locals, aliases, enum carriers and fixed-width controls on both plans.
// Clang verifies the complete emitted module when installed.
// Rules: rules/types/types.md — "int and uint", "Context shaping";
// rules/declarations/enums.md — "Underlying type"; correction5.md — scalar facts.
func TestLegacyLLVMTargetIntegers(t *testing.T) {
	data, err := os.ReadFile("../../../../testdata/codegen/llvm_target_integers/scalars.sec")
	if err != nil {
		t.Fatal(err)
	}
	bounds, err := os.ReadFile("../../../../testdata/codegen/llvm_target_integers/bounds.sec.in")
	if err != nil {
		t.Fatal(err)
	}
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
			boundSource := strings.NewReplacer("@SIGNED_MAX@", maximum, "@SIGNED_MIN@", minimum, "@UNSIGNED_MAX@", unsignedMax).Replace(string(bounds))
			parsed := parser.New(lexer.New(string(data) + boundSource)).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(plan)
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			output, err := llvm.GenerateAnalyzed(parsed.Program, analyzer, definition.LLVMTriple)
			if err != nil {
				t.Fatal(err)
			}
			native := "i32"
			if arch == "amd64" {
				native = "i64"
			}
			for _, want := range []string{
				"define " + native + " @SignedIdentity(" + native,
				"define " + native + " @UnsignedIdentity(" + native,
				"define " + native + " @EnumIdentity(" + native,
				"define " + native + " @DefaultEnumIdentity(" + native,
				"ret " + native + " " + maximum,
				"ret " + native + " " + minimum,
				"ret i64 -9223372036854775808",
				"ret i64 18446744073709551615",
				"ret " + native + " " + unsignedMax,
				"add " + native + " %value, 1", "sub " + native + " 0, %value",
				"store " + native + " 7", "store " + native + " 9", "ret " + native + " 7",
				"define i32 @Fixed(i32", "store i32 3", "store i32 4",
				"define i64 @FixedUnsigned(i64",
				"define " + native + " @Length(ptr",
			} {
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q:\n%s", want, output)
				}
			}
			if arch == "armv7" {
				for _, want := range []string{"sext i32 %value to i64", "zext i32 %value to i64"} {
					if !strings.Contains(output, want) {
						t.Fatalf("lost extension signedness: %s", output)
					}
				}
			}
			for _, generate := range []func() (string, error){
				func() (string, error) { return llvm.GenerateWithTriple(parsed.Program, definition.LLVMTriple) },
				func() (string, error) { return llvm.GenerateAnalyzed(parsed.Program, analyzer, definition.LLVMTriple) },
			} {
				actual, err := generate()
				if err != nil || actual != output {
					t.Fatal("entry points disagree", err)
				}
			}
			other, _ := platformtarget.Find(platformtarget.Target{OS: "linux", Arch: map[string]string{"armv7": "amd64", "amd64": "armv7"}[arch]})
			if partial, err := llvm.GenerateAnalyzed(parsed.Program, analyzer, other.LLVMTriple); err == nil || partial != "" {
				t.Fatal("mismatched Sema target accepted")
			}
			if clang, err := exec.LookPath("clang"); err == nil {
				file := filepath.Join(t.TempDir(), "module.ll")
				if err := os.WriteFile(file, []byte(output), 0600); err != nil {
					t.Fatal(err)
				}
				command := exec.Command(clang, "-x", "ir", "-c", file, "-target", definition.LLVMTriple, "-o", file+".o")
				if diagnostics, err := command.CombinedOutput(); err != nil {
					t.Fatalf("LLVM rejected target module: %v\n%s", err, diagnostics)
				}
			}
		})
	}
	parsed := parser.New(lexer.New(string(data))).Parse()
	output, err := llvm.GenerateWithTriple(parsed.Program, "unregistered-64bit-target")
	var unsupported *semantic.UnsupportedFeatureError
	if output != "" || !errors.As(err, &unsupported) {
		t.Fatal("unknown target guessed a width", output, err)
	}
}

// TestPlanRegistryParity proves width selection consumes exact canonical target
// identities, including non-host architectures; ambiguous spellings never guess.
// Rules: rules/projects/projects.md — "Compilation plans and target lowering".
func TestPlanRegistryParity(t *testing.T) {
	for _, definition := range platformtarget.Definitions() {
		if definition.LLVMTriple == "" {
			continue
		}
		got, err := targetplan.Plan(definition.LLVMTriple)
		want, wantErr := definition.ScalarPlan()
		if fmt.Sprint(err) != fmt.Sprint(wantErr) || got != want {
			t.Fatal(definition, got, err)
		}
	}
	if _, err := targetplan.Plan("x86_64-made-up-linux"); err == nil {
		t.Fatal("unknown triple accepted")
	}
}

// TestLegacyLLVMDefaultTarget consumes the same registry host identity for all
// default generator entry points instead of constructing a backend-only triple.
// Rules: rules/projects/projects.md — "Compilation plans and target lowering".
func TestLegacyLLVMDefaultTarget(t *testing.T) {
	host, ok := platformtarget.Find(platformtarget.Host())
	if !ok {
		t.Skip("host not registered")
	}
	plan, err := targetplan.Plan("")
	if err != nil || plan.LLVMTriple != host.LLVMTriple || plan.PointerWidthBits != host.PointerWidthBits {
		t.Fatal(plan, err)
	}
	data, err := os.ReadFile("../../../../testdata/codegen/llvm_target_integers/scalars.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.New(string(data))).Parse()
	want, err := llvm.GenerateWithTriple(parsed.Program, host.LLVMTriple)
	if err != nil {
		t.Fatal(err)
	}
	for _, generate := range []func() (string, error){func() (string, error) { return llvm.Generate(parsed.Program) }, func() (string, error) { return llvm.NewGenerator().Generate(parsed.Program) }} {
		got, err := generate()
		if err != nil || got != want {
			t.Fatal("default entry point disagrees with registry", err)
		}
	}
}
