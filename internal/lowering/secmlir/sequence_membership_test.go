package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
	"strings"
	"testing"
)

// The membership CFG emits existing scalar equality, bounds and slice
// operations without weakening authority or selecting physical slice layout.
// Rules: rules/foundations/operators.md — §A.14;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45–58,87.
func TestEmitSequenceMembership(t *testing.T) {
	const path = "../../../testdata/semantic_ir/sequence_membership/scalars.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if errors := p.Errors(); len(errors) != 0 {
			t.Fatal(errors)
		}
		a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		m, err := semantic.Build(program, a, semantic.BuildOptions{MaxPackage: 16})
		if err != nil {
			t.Fatal(err)
		}
		output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"sec.slice.element_place", "sec.slice.index_in_bounds", "sec.array.extract", "sec.float.cmp", "sec.enum.cmp"} {
			if !strings.Contains(string(output), text) {
				t.Errorf("missing %s in emission", text)
			}
		}
	}
}

// Native representation is not inferred from successful high-level emission:
// all emitted modules verify, but the single LLVM track must retain its named
// array/slice capability rejection and leave no artifact until physical lowering.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§87,142.
func TestSequenceMembershipDialectAndNativeBoundary(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	const path = "../../../testdata/semantic_ir/sequence_membership/scalars.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		m, err := semantic.Build(program, a, semantic.BuildOptions{MaxPackage: 16})
		if err != nil {
			t.Fatal(err)
		}
		output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
		if err != nil {
			t.Fatal(err)
		}
		valid := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-")
		valid.Stdin = bytes.NewReader(output)
		if diagnosis, err := valid.CombinedOutput(); err != nil {
			t.Fatalf("membership guards do not verify: %v\n%s", err, diagnosis)
		}
		artifact := filepath.Join(t.TempDir(), "membership.mlir")
		native := exec.Command(tool, "--sec-lower-to-llvm", "-o", artifact, "-")
		native.Stdin = bytes.NewReader(output)
		diagnosis, err := native.CombinedOutput()
		if err == nil || !bytes.Contains(diagnosis, []byte("unsupported-lowering")) {
			t.Fatalf("lost physical representation boundary: %v\n%s", err, diagnosis)
		}
		t.Logf("plan %d native boundary: %s", width, diagnosis)
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("failed native lowering wrote an artifact", err)
		}
	}
}
