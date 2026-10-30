package secmlir

import (
	"bytes"
	"os"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// Whole-self copy stays a typed Place read from a borrowed receiver, preserving
// named scalar/struct identities through the existing P15 schema and verifiers.
// Rules: rules/declarations/functions.md §22; rules/memory/copy_move.md §18;
// rules/compiler/semantic_ir.md §§60–61; rules/mlir/packages/sec-mlir-dialect_package15.md §59.
func TestPackage15EmitsTrivialReceiverValueCopies(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/receiver_value_copy.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "receiver-copy.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
		if errors := a.Analyze(program); len(errors) > 0 {
			t.Fatal(errors)
		}
		m, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 15})
		if err != nil {
			t.Fatal(err)
		}
		for _, schema := range []uint32{11, 12} {
			output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: schema})
			if err != nil {
				t.Fatal(err)
			}
			again, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: schema})
			if err != nil || !bytes.Equal(output, again) {
				t.Fatal("receiver copy is nondeterministic", err)
			}
			if !strings.Contains(string(output), `"sec.place.read"`) || strings.Contains(string(output), `"sec.ref.move"`) {
				t.Fatal("whole-self copy lost Place semantics")
			}
			verifyPackage15MLIR(t, "receiver_value_copy", output)
		}
	}
}
