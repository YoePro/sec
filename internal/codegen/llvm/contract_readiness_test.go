package llvm

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ir/semantic"
	"sec/internal/sema"
)

// TestLegacyLLVMContractReadiness closes every raw entry point for all contract
// families, unchecked mutation, and unknown nominal representations.
// Rules: rules/types/contracts.md — inventory, Mutation, Conversion failure layers.
func TestLegacyLLVMContractReadiness(t *testing.T) {
	for _, name := range []string{"range", "membership", "multiple", "parity", "finite", "length", "markers", "nested", "regex_invalid", "fallible", "unknown", "unknown_constructor_invalid", "cycle_invalid"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../../testdata/codegen/llvm_contracts/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			program := parseProgram(t, string(data))
			for _, generate := range []func(*ast.Program) (string, error){Generate, NewGenerator().Generate, func(p *ast.Program) (string, error) { return GenerateWithTriple(p, "x86_64-pc-linux-gnu") }} {
				output, err := generate(program)
				var failure *semantic.UnsupportedFeatureError
				if output != "" || !errors.As(err, &failure) || failure.Location.Line == 0 {
					t.Fatal(output, err)
				}
			}
		})
	}
}

// TestLegacyLLVMAnalyzedContracts checks inherited and nested imported facts
// even when their declarations are absent from the requested output AST.
// Rules: rules/types/contracts.md — Composition and Core rule.
func TestLegacyLLVMAnalyzedContracts(t *testing.T) {
	for _, name := range []string{"range", "nested"} {
		data, err := os.ReadFile("../../../testdata/codegen/llvm_contracts/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		program := parseProgram(t, string(data))
		analyzer := sema.NewAnalyzer()
		if problems := analyzer.Analyze(program); len(problems) != 0 {
			t.Fatal(problems)
		}
		outputRoot := &ast.Program{}
		for _, statement := range program.Statements {
			if _, declaration := statement.(*ast.TypeDeclStatement); !declaration {
				outputRoot.Statements = append(outputRoot.Statements, statement)
			}
		}
		output, err := GenerateAnalyzed(outputRoot, analyzer, "x86_64-pc-linux-gnu")
		var failure *semantic.UnsupportedFeatureError
		if output != "" || !errors.As(err, &failure) || !strings.Contains(failure.Feature, "type contract validation") {
			t.Fatal(output, err)
		}
	}
}

// TestLegacyLLVMFailedGenerationCannotLeak verifies retrying the same generator
// cannot publish a function whose unresolved representation failed readiness.
// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites.
func TestLegacyLLVMFailedGenerationCannotLeak(t *testing.T) {
	g := NewGenerator()
	for _, name := range []string{"unknown", "plain"} {
		data, err := os.ReadFile("../../../testdata/codegen/llvm_contracts/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		output, err := g.Generate(parseProgram(t, string(data)))
		if name == "unknown" {
			if output != "" || err == nil {
				t.Fatal(output, err)
			}
			continue
		}
		if err != nil || strings.Count(output, "@Identity(") != 1 || strings.Contains(output, "define void @Identity") {
			t.Fatal(output, err)
		}
	}
}
