package secmlir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sec/internal/codegen/targetplan"
	semantic "sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// TestPackage15HighLevelOptimizationBoundary checks the executable P15 boundary:
// source proofs survive available high-level transforms, while physical lowering
// remains rejected. Later representation consumers own their LLVM differential
// tests; their absence cannot require P15 to select a physical representation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md — §§150–155;
// rules/corrections/applied/md015-md061-duration-reborrow-correction-20261015.md
// — §§6(1–8),7(5); rules/mlir/packages/sec-mlir-dialect_package26.md — §53.
func TestPackage15HighLevelOptimizationBoundary(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	const source = "../../../testdata/semantic_ir/reference_shared_call_reborrow.sec"
	input, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	verifiers := "func.func(sec-verify-places,sec-verify-reference-guards,sec-verify-borrow-semantics)"
	pipeline := "builtin.module(" + verifiers + ",canonicalize," + verifiers + ",cse," + verifiers + ")"
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			plan, err := targetplan.Plan(testPlan(width).LLVMTriple)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(input), source))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := sema.NewAnalyzerWithScalarPlan(plan)
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{source}, MaxPackage: 15})
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
			original := filepath.Join(t.TempDir(), "source.mlir")
			if err := os.WriteFile(original, output, 0600); err != nil {
				t.Fatal(err)
			}
			for _, stage := range []string{"unlowered", "sec-lower-scalar-core", "sec-lower-integer-core"} {
				t.Run(stage, func(t *testing.T) {
					before := filepath.Join(t.TempDir(), "before.mlir")
					args := []string{original}
					if stage != "unlowered" {
						args = append(args, "--"+stage)
					}
					args = append(args, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-o", before)
					if data, err := exec.Command(tool, args...).CombinedOutput(); err != nil {
						t.Fatalf("baseline verification: %v\n%s", err, data)
					}
					after := filepath.Join(t.TempDir(), "after.mlir")
					if data, err := exec.Command(tool, before, "--pass-pipeline="+pipeline, "--verify-each", "-o", after).CombinedOutput(); err != nil {
						t.Fatalf("optimization changed reference invariants: %v\n%s", err, data)
					}
					for _, path := range []string{before, after} {
						text, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						for _, forbidden := range []string{"!llvm.ptr", "llvm.noalias", "llvm.nonnull", "dereferenceable", "llvm.intr.lifetime"} {
							if strings.Contains(string(text), forbidden) {
								t.Fatalf("P15 fabricated physical proof %q", forbidden)
							}
						}
						data, err := exec.Command(tool, path, "--sec-check-lowering-support", "-o", os.DevNull).CombinedOutput()
						if err == nil || !strings.Contains(string(data), "unsupported-lowering") {
							t.Fatalf("unresolved reference representation passed physical gate: %v\n%s", err, data)
						}
					}
				})
			}
		})
	}
}

// Existing CFG regressions exercise epoch guards, holder lifetimes and transfer
// dependencies. Legal inputs verify again after canonicalize and CSE; malformed
// inputs must fail their verifier before an optimizer can hide the violation.
// Rules: MD-061 correction §§6(3–7),7(5); P15 §§82–85,131,154.
func TestPackage15OptimizationDependencyCFG(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, test := range []struct{ file, verifier string }{
		{"reference-guard-cfg", "sec-verify-reference-guards"},
		{"reference-lifetime-cfg", "sec-verify-borrow-semantics"},
		{"reference-transfer-cfg", "sec-verify-borrow-semantics"},
		{"reference-transfer-validity", "sec-verify-borrow-semantics"},
	} {
		for _, invalid := range []bool{false, true} {
			name := test.file
			if invalid {
				name = "invalid-" + name
			}
			t.Run(name, func(t *testing.T) {
				path := filepath.Join("../../../mlir/test/Dialect/Sec", name+".mlir")
				verify := "func.func(" + test.verifier + ")"
				pipeline := "builtin.module(" + verify + ",canonicalize," + verify + ",cse," + verify + ")"
				output := filepath.Join(t.TempDir(), "result.mlir")
				args := []string{path, "-split-input-file", "--verify-diagnostics", "--pass-pipeline=" + pipeline, "--verify-each", "-o", output}
				if data, err := exec.Command(tool, args...).CombinedOutput(); err != nil {
					t.Fatalf("dependency diagnostics/invariants changed: %v\n%s", err, data)
				}
				if invalid {
					data, err := os.ReadFile(output)
					if err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					source, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					// Negative diagnostic files can include valid control chunks.
					// Only those controls may emit a module; rejected chunks emit
					// at most the split-input-file driver's separator comment.
					controls := 0
					for _, chunk := range strings.Split(string(source), "// -----") {
						// A comments-only header chunk is a valid empty
						// implicit module in the MLIR parser as well.
						if !strings.Contains(chunk, "expected-error") {
							controls++
						}
					}
					if got := strings.Count(string(data), "module {"); got != controls {
						t.Fatalf("emitted %d modules; only %d valid control chunks may emit", got, controls)
					}
				}
			})
		}
	}
}
