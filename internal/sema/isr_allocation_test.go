package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestISRAllocationGuarantees verifies the allocation portion of ISR contracts
// through canonical summaries without enabling unsupported ISR semantics.
// Rules: rules/foundations/attributes.md — "Attribute implications", "ISR
// verification", "Interrupt-safe guarantee", "@noAlloc verification".
func TestISRAllocationGuarantees(t *testing.T) {
	const file = "../../testdata/sema/isr_allocation_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		want := map[string]struct {
			root, cause int
			unknown     bool
		}{
			"Direct": {11, 4, false}, "Bound": {13, 14, true},
			"Helper": {15, 4, false}, "Redundant": {18, 4, false},
			"Deferred": {20, 4, false}, "Unknown": {22, 23, true},
			"ForeignRoot": {24, 25, true},
			"Local":       {30, 33, false},
		}
		unsupported := 0
		for _, e := range a.Analyze(program) {
			if strings.Contains(e.Message, "not implemented yet") {
				unsupported++
			}
			if e.ID != diagnostics.NoAllocViolation {
				continue
			}
			found := false
			for name, expected := range want {
				if !strings.Contains(e.Message, "function "+name+" ") {
					continue
				}
				found = true
				if e.Line != expected.root || e.PreviousLine != expected.cause || e.File != file || e.PreviousFile != file || e.AllocationCause == nil || !strings.Contains(e.Help, "unsafe does not waive") || strings.Contains(e.Help, "remove @noAlloc") {
					t.Fatalf("%s/%s: %+v", depth, name, e)
				}
				g := a.CallGraph()
				summary := g.ArenaSummary(callGraphNodeIDByName(t, g, name))
				if summary.MayAllocate == expected.unknown || summary.AllocationUnknown != expected.unknown {
					t.Fatal(name, summary)
				}
				delete(want, name)
				break
			}
			if !found {
				t.Fatal("unexpected violation", e)
			}
		}
		if len(want) != 0 || unsupported != 10 {
			t.Fatal(depth, want, unsupported, a.errors)
		}
		for _, name := range []string{"Proven", "ContractProven"} {
			g := a.CallGraph()
			summary := g.ArenaSummary(callGraphNodeIDByName(t, g, name))
			if summary.MayAllocate || summary.AllocationUnknown {
				t.Fatal(name, summary)
			}
		}
	}
}
