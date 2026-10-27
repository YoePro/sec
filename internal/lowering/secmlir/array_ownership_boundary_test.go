package secmlir

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// A represented owned array must not be emitted with P14's copy-only metadata.
// The source-to-IR stage works; schema 13 remains its explicit successor gate.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 65, 79, 128.
func TestEmitOwnedArrayRequiresSchema13(t *testing.T) {
	input, err := os.ReadFile("../../../testdata/semantic_ir/owned_fixed_arrays.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(input)))
	program := p.ParseProgram()
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	output, err := Emit(module, testPlan(64))
	var unsupported *UnsupportedLoweringError
	if output != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "schema 13") {
		t.Fatalf("owned array metadata erased: output=%q error=%v", output, err)
	}
}
