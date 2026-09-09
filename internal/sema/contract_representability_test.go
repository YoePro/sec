package sema

import (
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestIntegerContractRepresentableConjunction(t *testing.T) {
	for _, test := range []struct {
		file string
		want []string
	}{
		{"representable_conjunction_valid.sec", nil},
		{"representable_conjunction_invalid.sec", []string{"ImpossibleUnsigned", "ImpossibleSigned", "ImpossibleInherited", "ImpossibleReordered"}},
	} {
		t.Run(test.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/contracts/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			errors := analyzeSource(t, string(source))
			if len(errors) != len(test.want) {
				t.Fatalf("errors = %v, want %d unsatisfiable declarations", errors, len(test.want))
			}
			for _, name := range test.want {
				if !errorsContainMessage(errors, "contracts cannot be satisfied together for "+name) {
					t.Errorf("missing consistency diagnostic for %s: %v", name, errors)
				}
			}
		})
	}
}

func TestIntegerContractRepresentabilityUsesTarget(t *testing.T) {
	source, err := os.ReadFile("../../testdata/contracts/representable_conjunction_target.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.New(string(source)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		errors := a.Analyze(program)
		if width == 64 {
			if len(errors) != 0 {
				t.Fatalf("64-bit contracts should be satisfiable: %v", errors)
			}
			continue
		}
		if len(errors) != 2 || !errorsContainMessage(errors, "contracts cannot be satisfied together for TargetSigned") || !errorsContainMessage(errors, "contracts cannot be satisfied together for TargetUnsigned") {
			t.Fatalf("32-bit target should reject only target-sized types: %v", errors)
		}
	}
}
