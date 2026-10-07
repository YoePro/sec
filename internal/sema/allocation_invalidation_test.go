package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAllocationSummaryReanalysis rejects stale body/foreign-contract effects
// when one Analyzer is reused for a newly parsed source snapshot. Historical
// detached facts remain unchanged while current routes and sites are replaced.
// Rules: rules/memory/allocation.md — §§24(6),29(5).
func TestAllocationSummaryReanalysis(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			a := NewAnalyzerWithDepth(depth)
			for _, scenario := range []struct {
				file      string
				knowledge AllocationKnowledge
			}{{"helper_trusted", AllocationFree}, {"helper_untrusted", AllocationUnknown}, {"helper_allocating", AllocationMayAllocate}, {"helper_free", AllocationFree}} {
				before := a.AllocationFacts()
				snapshot := a.AllocationFacts()
				data, err := os.ReadFile("../../testdata/lsp/allocation_refresh/" + scenario.file + ".sec")
				if err != nil {
					t.Fatal(err)
				}
				p := parser.New(lexer.NewWithFile(string(data), "imported-helper.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				if errs := a.Analyze(program); len(errs) != 0 {
					t.Fatal(errs)
				}
				if !reflect.DeepEqual(before, snapshot) {
					t.Fatal("previous snapshot was mutated")
				}
				id := callGraphNodeIDByName(t, a.CallGraph(), "Work")
				fact := a.AllocationFact(id)
				if fact.Knowledge != scenario.knowledge {
					t.Fatal(scenario, fact)
				}
				contexts := a.AllocationContexts(id)
				if scenario.knowledge == AllocationFree && len(contexts) != 0 {
					t.Fatal("stale allocation contexts", contexts)
				}
				cause := a.CallGraph().AllocationCause(id)
				if scenario.knowledge == AllocationFree && len(cause.Steps) != 0 {
					t.Fatal("stale allocation cause", cause)
				}
				if scenario.knowledge == AllocationUnknown && (!cause.Unknown || len(cause.Steps) == 0) {
					t.Fatal(cause)
				}
			}
		})
	}
}
