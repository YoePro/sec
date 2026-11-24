package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// Layout failures reach the source operation and shared CLI/LSP presentation;
// successful queries do not publish a spurious failure or a guessed alignment.
// Rules: MD-141 correction §10(1); rules/memory/layout.md — §§5,42.
func TestArenaLayoutFailurePresentation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/layout_failure_context.sec")
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	good, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing", "invalid", "complete"} {
		t.Run(mode, func(t *testing.T) {
			plan := good
			plan.Alignments = layout.ScalarAlignments{}
			for key, value := range good.Alignments {
				plan.Alignments[key] = value
			}
			switch mode {
			case "missing":
				delete(plan.Alignments, layout.StorageRepresentation{Class: layout.BinaryFloatStorage, WidthBits: 64})
			case "invalid":
				plan.Alignments[layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: 128}] = 3
			}
			p := parser.New(lexer.NewWithFile(string(data), "layout_failure_context.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(plan)
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			failures := 0
			for _, fact := range a.AllocationContextFacts() {
				if fact.LayoutFailure == "" {
					continue
				}
				failures++
				message := AllocationContextDescription(fact)
				for _, want := range []string{"layout of", "linux/amd64", good.LLVMTriple, "profile hosted", "ABI " + good.ABI, "needs a complete layout"} {
					if !strings.Contains(message, want) {
						t.Errorf("missing %q: %s", want, message)
					}
				}
				if mode == "missing" && (!strings.Contains(message, "float64") || !strings.Contains(message, "Arena.New") || !strings.Contains(message, "supply verified target")) {
					t.Fatal(message)
				}
				if mode == "invalid" && !strings.Contains(message, "correct the contradictory") {
					t.Fatal(message)
				}
			}
			want := 0
			if mode == "missing" {
				want = 1
			}
			if mode == "invalid" {
				want = 2
			}
			if failures != want {
				t.Fatalf("%d failures, want %d", failures, want)
			}
			if failures > 0 {
				err := a.ValidateAllocationLoweringReadiness(program, "main")
				capability, ok := err.(*AllocationLoweringError)
				if !ok || !strings.Contains(capability.ImplementationCapability().Feature, "layout of float64") {
					t.Fatalf("lowering readiness lost the layout failure or its capability category: %v", err)
				}
			}
		})
	}
}
