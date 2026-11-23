package secmlir

import (
	"errors"
	"os"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// High-level runtime string plans verify, but cannot produce MLIR artifacts
// until physical string allocation, encoding and failure endpoints are lowered.
// Rules: rules/compiler/semantic_ir.md — §§17(6),102–103; MD-039 §7(3).
func TestMD039StringMaterializationBoundary(t *testing.T) {
	for _, fixture := range []string{"runtime_concat.sec", "fallible_operands.sec"} {
		t.Run(fixture, func(t *testing.T) {
			source, err := os.ReadFile("../../../testdata/semantic_ir/md039/" + fixture)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(source), "runtime_concat.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			analyzer := sema.NewAnalyzer()
			if errs := analyzer.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			module, err := semantic.Build(program, analyzer, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 19})
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 16})
			var boundary *UnsupportedLoweringError
			if len(output) != 0 || !errors.As(err, &boundary) || boundary.Feature != string(semantic.OpStringMaterialize) {
				t.Fatalf("expected named materialization boundary and no artifact: %s %v", output, err)
			}
		})
	}
}
