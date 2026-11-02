package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MD-086: the inspection routes reach their own verified boundaries for the
// package-15 to package-17 subset — emit-ir publishes verified Semantic IR and
// emit-sec-mlir verified schema-13 Sec MLIR (the Place, reference-guard and
// borrow-semantics verifiers included) — while emit-mlir and emit-llvm, which
// need the still-missing physical reference representation, reject the same
// valid program as an unsupported lowering, print nothing and leave an
// existing output file untouched.
//
// Rules:
//   - rules/compiler/compiler_pipeline.md — § 59(4)–(7) (MD-086)
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 3, 155
func TestReferenceProgramsReachOnlyTheInspectionBoundaries(t *testing.T) {
	file := "../../testdata/semantic_ir/reference_emit_boundary.sec"
	if _, stderr, code := runCLIForDiagnostics(t, "check", file); code != 0 {
		t.Fatal(code, stderr)
	}
	t.Run("emit-ir", func(t *testing.T) {
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file)
		if code != 0 {
			t.Fatal(code, stderr)
		}
		for _, want := range []string{"ref.borrow-shared", "ref.deref-place", "place.read", "[shared-reference]"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("emit-ir lacks %q:\n%s", want, stdout)
			}
		}
	})
	t.Run("emit-sec-mlir", func(t *testing.T) {
		bin := os.Getenv("SEC_MLIR_BIN")
		if bin == "" {
			t.Skip("SEC_MLIR_BIN is not set")
		}
		for _, target := range []string{"linux-amd64", "linux-armv7"} {
			stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", target, "--mlir-bin", bin)
			if code != 0 {
				t.Fatal(target, code, stderr)
			}
			for _, want := range []string{"sec.dialect_version = 13 : i32", `"sec.ref.borrow_shared"(`, `sec.argument_actions = ["shared-reference"]`, `"sec.place.deref"(`} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s emit-sec-mlir lacks %q:\n%s", target, want, stdout)
				}
			}
		}
	})
	for _, command := range []string{"emit-llvm", "emit-mlir"} {
		t.Run(command, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := runCLIForDiagnostics(t, command, file, "-o", path)
			if code == 0 || stdout != "" || !strings.Contains(stderr, "physical representation of Places, safe references and ownership cleanup") || !strings.Contains(stderr, "unsupported-lowering") {
				t.Fatal(code, stdout, stderr)
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != "existing output" {
				t.Fatal(string(contents), err)
			}
		})
	}
}

// MD-086 § 6(2)–(7): the inspection routes publish verified output for the
// wider package-15 matrix — shared/mutable borrows, reborrows, field and
// element projections, reference equality, whole-referent reads and writes,
// call-lifetime temporaries and returned field origins — deterministically
// and labelled schema 13 on 32- and 64-bit plans. Invalid source, invalid
// return origins and features beyond the package-15 subset publish nothing.
func TestReferenceInspectionRoutesCoverThePackage15Matrix(t *testing.T) {
	file := "../../testdata/semantic_ir/reference_emit_matrix.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	for _, want := range []string{"ref.borrow-shared", "ref.borrow-mut", "ref.reborrow-shared", "ref.copy-shared", "ref.compare", "ref.deref-place", "place.field", "place.array-element", "place.write", "place.value-root", "returns-from=[param#0.field#1]"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("emit-ir lacks %q:\n%s", want, stdout)
		}
	}
	bin := os.Getenv("SEC_MLIR_BIN")
	if bin == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		first, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", target, "--mlir-bin", bin)
		if code != 0 {
			t.Fatal(target, code, stderr)
		}
		again, _, _ := runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", target, "--mlir-bin", bin)
		if first != again {
			t.Fatalf("%s emit-sec-mlir is not deterministic", target)
		}
		for _, want := range []string{"sec.dialect_version = 13 : i32", `"sec.ref.reborrow_shared"(`, `"sec.ref.compare"(`, `sec.return_origins = ["param#0.field#1"]`, `sec.argument_actions = ["mutable-reference"]`, "sec.synthesized = true", `"sec.place.write"(`} {
			if !strings.Contains(first, want) {
				t.Fatalf("%s emit-sec-mlir lacks %q:\n%s", target, want, first)
			}
		}
	}

	directory := t.TempDir()
	for name, test := range map[string]struct{ source, want string }{
		"invalid-borrow": {`module main

fn main() int {
    let mut n := 1
    let r := ref mut n
    let s := ref n
    *r = 2
    return *s
}
`, "sema error"},
		"escaping-origin": {`module main

fn Escape() ref int {
    let local := 1
    return ref local
}

fn main() int {
    return 0
}
`, "cannot return reference to local variable local"},
		"beyond-package-16": {`module main
type Holder struct {
    View: ref int[],
}
fn Read(data: ref int[]) int {
    let holder := Holder { View: data }
    return holder.View[0]
}
`, "unsupported-lowering"},
	} {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(directory, name+".sec")
			if err := os.WriteFile(input, []byte(test.source), 0600); err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{"emit-ir", "emit-sec-mlir"} {
				path := filepath.Join(t.TempDir(), "output")
				if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{command, input, "-o", path}
				if command == "emit-sec-mlir" {
					args = append(args, "--mlir-bin", bin)
				}
				stdout, stderr, code := runCLIForDiagnostics(t, args...)
				if code == 0 || stdout != "" || !strings.Contains(stderr, test.want) {
					t.Fatalf("%s %s: code=%d stdout=%q stderr=%s", command, name, code, stdout, stderr)
				}
				if contents, err := os.ReadFile(path); err != nil || string(contents) != "existing output" {
					t.Fatalf("%s %s replaced the existing output: %q %v", command, name, contents, err)
				}
			}
		})
	}
}

// Borrowed slices reach the inspection routes through the single track, with the
// slice guard verifier, and stop at a named boundary on the native routes.
// Rules: rules/compiler/compiler_pipeline.md §59; rules/mlir/packages/sec-mlir-dialect_package16.md §144.
func TestSliceProgramsReachOnlyTheInspectionBoundaries(t *testing.T) {
	bin := os.Getenv("SEC_MLIR_BIN")
	if bin == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	file := filepath.Join("..", "..", "testdata", "semantic_ir", "slice_source_branch_ends.sec")
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", target, "-o", "-")
		if code != 0 || !strings.Contains(stdout, "slice.reborrow-mut") {
			t.Fatal(target, code, stderr)
		}
		stdout, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", target, "--mlir-bin", bin)
		if code != 0 {
			t.Fatal(target, code, stderr)
		}
		for _, want := range []string{"sec.dialect_version = 13 : i32", `"sec.slice.reborrow_mut"(`, "sec.ref.end_borrow"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%s emit-sec-mlir lacks %q:\n%s", target, want, stdout)
			}
		}
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-mlir", file, "--target", "linux-amd64", "--mlir-bin", bin, "-o", "-")
	if code != 4 || stdout != "" || !strings.Contains(stderr, "[unsupported-lowering]") {
		t.Fatal(code, stdout, stderr)
	}
}

// Ownership programs stop at the inspection boundaries: emit-sec-mlir publishes
// schema 13 after all three P17 verifiers, and lowering names the missing
// physical cleanup instead of failing on the source. Output is deterministic.
// Rules: rules/compiler/compiler_pipeline.md — § 59(4); rules/mlir/packages/sec-mlir-dialect_package17.md — §§114-116.
func TestOwnershipProgramsReachOnlyTheInspectionBoundaries(t *testing.T) {
	bin := os.Getenv("SEC_MLIR_BIN")
	if bin == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	file := filepath.Join("..", "..", "testdata", "semantic_ir", "ownership_source_core.sec")
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", target, "-o", "-")
		if code != 0 || !strings.Contains(stdout, "cleanup.track-owned") {
			t.Fatal(target, code, stderr)
		}
		stdout, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", target, "--mlir-bin", bin)
		if code != 0 {
			t.Fatal(target, code, stderr)
		}
		for _, want := range []string{"sec.dialect_version = 13 : i32", "sec.destruction_plans = [", `"sec.own.move"(`, `"sec.cleanup.run_function"(`, `"sec.cleanup.defer_register"(`} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%s emit-sec-mlir lacks %q:\n%s", target, want, stdout)
			}
		}
		again, _, code := runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", target, "--mlir-bin", bin)
		if code != 0 || again != stdout {
			t.Fatal(target, "emission is not deterministic")
		}
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-mlir", file, "--target", "linux-amd64", "--mlir-bin", bin, "-o", "-")
	if code != 4 || stdout != "" || !strings.Contains(stderr, "ownership cleanup") {
		t.Fatal(code, stdout, stderr)
	}
}
