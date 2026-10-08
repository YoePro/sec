package llvm

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ir/semantic"
)

// TestLegacyLLVMRejectsPlatformFloat checks all generator entry points before
// output, including inferred family literals and nested or unused type sites.
// Rules: rules/types/types.md — "Binary floating-point types"; MD-014 §6.
func TestLegacyLLVMRejectsPlatformFloat(t *testing.T) {
	for _, name := range []string{"signature", "conversion", "alias", "field", "nested", "default", "inferred_fraction", "inferred_integer"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile("../../../testdata/codegen/llvm_platform_float/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			program := parseProgram(t, string(input))
			for _, generate := range []func(*ast.Program) (string, error){Generate, NewGenerator().Generate,
				func(p *ast.Program) (string, error) { return GenerateWithTriple(p, "armv7-unknown-linux-gnueabihf") },
				func(p *ast.Program) (string, error) { return GenerateWithTriple(p, "x86_64-pc-linux-gnu") }} {
				output, err := generate(program)
				var failure *semantic.UnsupportedFeatureError
				if output != "" || !errors.As(err, &failure) || !strings.Contains(failure.Feature, "float") || failure.Location.Line == 0 {
					t.Fatalf("output=%q error=%v", output, err)
				}
			}
		})
	}
}

// TestLegacyLLVMExplicitFloatWidths preserves fixed-width signatures without
// selecting a width from the backend target triple.
// Rules: rules/types/types.md — "Binary floating-point types".
func TestLegacyLLVMExplicitFloatWidths(t *testing.T) {
	input, err := os.ReadFile("../../../testdata/codegen/llvm_platform_float/explicit_widths.sec")
	if err != nil {
		t.Fatal(err)
	}
	program := parseAndAnalyze(t, string(input))
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		output, err := GenerateWithTriple(program, triple)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{"define float @Narrow(float", "ret float %value", "define double @Wide(double", "ret double %value"} {
			if !strings.Contains(output, expected) {
				t.Fatalf("missing %q in %s", expected, output)
			}
		}
	}
}
