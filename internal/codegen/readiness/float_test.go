package readiness_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: types/types.md — Binary floating-point types; MD-014 correction §6.
func TestLegacyMLIRRejects32BitPlatformFloat(t *testing.T) {
	plan, err := targetplan.Plan("armv7-unknown-linux-gnueabihf")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"signature", "conversion", "alias", "field", "nested", "default", "inferred_fraction", "inferred_integer"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../../testdata/codegen/llvm_platform_float/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			parsed := parser.New(lexer.NewWithFile(string(source), name+".sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(plan)
			if errors := analyzer.Analyze(parsed.Program); len(errors) > 0 {
				t.Fatal(errors)
			}
			for _, generate := range []func(*ast.Program) (string, error){
				func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, plan.LLVMTriple) },
				func(p *ast.Program) (string, error) { return mlir.GenerateAnalyzed(p, analyzer, plan.LLVMTriple) },
			} {
				output, err := generate(parsed.Program)
				var unsupported *semantic.UnsupportedFeatureError
				if output != "" || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "float") || unsupported.Location.File != name+".sec" || unsupported.Location.Line == 0 {
					t.Fatalf("incorrect rejection: %q %v", output, err)
				}
			}
		})
	}
}

// Rules: types/types.md — Binary floating-point types, explicit widths.
func TestLegacyMLIRPreservesExplicitFloatSignatures(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/codegen/llvm_platform_float/explicit_widths.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		t.Run(triple, func(t *testing.T) {
			parsed := parser.New(lexer.New(string(source))).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			output, err := mlir.GenerateWithTriple(parsed.Program, triple)
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range []string{"@Narrow(%value: f32) -> f32", "@Wide(%value: f64) -> f64"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("missing %q: %s", expected, output)
				}
			}
		})
	}
}
