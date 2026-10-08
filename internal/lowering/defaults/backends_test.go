package defaults_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/codegen/llvm"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/lowering/secmlir"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// TestDefaultBackendConformance follows the same source defaults through both
// maintained output paths, pinning supported values and explicit capability
// boundaries. It never treats semantic acceptance as proof of runtime support.
// Rules: rules/types/default_values.md — Backend tests, Backend lowering, A.8;
// rules/mlir/packages/sec-mlir-dialect_package13.md — §26;
// rules/mlir/packages/sec-mlir-dialect_package14.md — §§24–27.
func TestDefaultBackendConformance(t *testing.T) {
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		for _, fixture := range []string{"scalars", "aggregates", "contracts", "owning", "list", "slice", "floating", "decimal", "string"} {
			t.Run(triple+"/"+fixture, func(t *testing.T) {
				source, err := os.ReadFile("../../../testdata/defaults/backends/" + fixture + ".sec")
				if err != nil {
					t.Fatal(err)
				}
				parsed := parser.New(lexer.New(string(source))).Parse()
				if parsed.HasErrors {
					t.Fatal(parsed.Diagnostics)
				}
				plan, err := targetplan.Plan(triple)
				if err != nil {
					t.Fatal(err)
				}
				analyzer := sema.NewAnalyzerWithScalarPlan(plan)
				diagnostics := analyzer.Analyze(parsed.Program)
				if fixture == "slice" {
					if len(diagnostics) != 1 || diagnostics[0].ID != "S1009" {
						t.Fatal(diagnostics)
					}
					return
				}
				if len(diagnostics) != 0 {
					t.Fatal(diagnostics)
				}
				if fixture == "contracts" {
					for name, want := range map[string]string{"Positive": "3", "Members": "7", "Override": "42"} {
						got, _, ok := sema.DefaultValueDisplay(analyzer.Types()[name])
						if !ok || got != want {
							t.Fatalf("%s default=%q, want %q", name, got, want)
						}
					}
				}
				t.Run("llvm", func(t *testing.T) {
					output, err := llvm.GenerateAnalyzed(parsed.Program, analyzer, triple)
					if fixture != "scalars" && fixture != "decimal" {
						want := map[string]string{"aggregates": "unresolved type representation", "contracts": "contract", "owning": "unresolved type representation", "list": "list", "floating": "float", "string": "string"}[fixture]
						if err == nil || output != "" || !strings.Contains(err.Error(), want) {
							t.Fatalf("unsupported default emitted output: %v\n%s", err, output)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					verifyLLVM(t, output, triple, fixture, plan.PointerWidthBits == 64)
				})
				t.Run("sec-mlir", func(t *testing.T) {
					module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main"})
					if fixture == "owning" || fixture == "list" || fixture == "contracts" || fixture == "string" {
						want := map[string]string{"owning": "dynamic", "list": "list", "contracts": "contracts", "string": "non-trivial"}[fixture]
						if err == nil || module != nil || !strings.Contains(err.Error(), want) {
							t.Fatalf("unsupported default built module: %v", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if err := semantic.Verify(module); err != nil {
						t.Fatal(err)
					}
					checkDefaultFacts(t, module, fixture)
					output, err := secmlir.Emit(module, plan)
					if err != nil {
						t.Fatal(err)
					}
					text := string(output)
					if strings.Contains(text, "undef") || strings.Contains(text, "poison") {
						t.Fatal(text)
					}
					if fixture == "aggregates" && (!strings.Contains(text, "sec.array.default") || !strings.Contains(text, "sec.struct.construct")) {
						t.Fatal(text)
					}
					t.Run("physical-verification", func(t *testing.T) {
						tool := testsupport.RequireSecMLIROptPath(t)
						path := filepath.Join(t.TempDir(), "defaults.mlir")
						if err := os.WriteFile(path, output, 0600); err != nil {
							t.Fatal(err)
						}
						lowered, err := exec.Command(tool, path, "--sec-verify-array-index-guards", "--sec-lower-scalar-core", "--sec-lower-trivial-core", "--verify-each").CombinedOutput()
						if err != nil {
							t.Fatalf("%v\n%s\n%s", err, lowered, output)
						}
						if strings.Contains(string(lowered), "undef") || strings.Contains(string(lowered), "poison") {
							t.Fatal(string(lowered))
						}
					})
				})
			})
		}
	}
}

// verifyLLVM compiles both target widths and executes observable scalar defaults
// on the host. A C caller checks values after loading their synthesized storage.
// Rules: rules/types/default_values.md — Backend tests and Mutable declarations.
func verifyLLVM(t *testing.T, output, triple, fixture string, execute bool) {
	t.Helper()
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "defaults.ll")
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(output, "@main(", "@SecEntry(")), 0600); err != nil {
		t.Fatal(err)
	}
	object := filepath.Join(dir, "defaults.o")
	diagnostics, err := exec.Command(clang, "-x", "ir", path, "-target", triple, "-c", "-o", object).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s\n%s", err, diagnostics, output)
	}
	if !execute {
		return
	}
	harness, err := os.ReadFile("../../../testdata/defaults/backends/" + fixture + ".c")
	if err != nil {
		t.Fatal(err)
	}
	caller := filepath.Join(dir, "caller.c")
	if err := os.WriteFile(caller, harness, 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "defaults")
	diagnostics, err = exec.Command(clang, caller, object, "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, diagnostics)
	}
	diagnostics, err = exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("incorrect default during execution: %v\n%s", err, diagnostics)
	}
}

// checkDefaultFacts checks materialized values and complete aggregate operands,
// including explicit default precedence and compact zero-length array defaults.
// Rules: rules/types/default_values.md — Backend tests, A.7–A.9;
// rules/mlir/packages/sec-mlir-dialect_package14.md — §§24–27.
func checkDefaultFacts(t *testing.T, module *semantic.Module, fixture string) {
	t.Helper()
	arrays := map[string]int{}
	structs, explicit := 0, 0
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				switch op.Kind {
				case semantic.OpConstInt:
					if op.Integer != nil && op.Integer.String() == "37" {
						explicit++
					}
				case semantic.OpStructConstruct:
					structs++
					if len(op.Operands) != 2 || len(op.StructOrigins) != 2 || len(op.StructActions) != 2 {
						t.Fatalf("incomplete nested aggregate: %#v", op)
					}
				case semantic.OpArrayDefault:
					arrays[op.ArrayLength]++
					if len(op.Operands) != 0 {
						t.Fatalf("expanded array default: %#v", op)
					}
				}
			}
		}
	}
	if fixture == "scalars" && explicit != 1 {
		t.Fatalf("explicit scalar default lost: %d", explicit)
	}
	if fixture == "aggregates" && (structs != 6 || explicit != 3 || arrays["4"] != 1 || arrays["3"] != 1 || arrays["0"] != 1) {
		t.Fatalf("missing aggregate defaults: structs=%d explicit=%d arrays=%v", structs, explicit, arrays)
	}
}
