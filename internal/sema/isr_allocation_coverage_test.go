package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestISRAllocationCoverage checks mandatory proof from canonical operation and
// cleanup coverage, recursion/defer/methods, positive violation precedence and
// worker-context separation, without enabling unsupported ISR attributes.
// Rules: rules/platform/interrupts.md — §§24–29;
// rules/analysis/isr_analysis.md — §§10.1–10.2,19,22,25.2–25.3.
func TestISRAllocationCoverage(t *testing.T) {
	const file = "../../testdata/sema/isr_allocation_coverage_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, profile := range []string{"hosted", "noalloc", "freestanding"} {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			a.SetAllocationProfile(profile)
			errors := a.Analyze(program)
			want := map[string]int{"DirectClosure": 1, "BoundRoot": 3, "RecursiveRoot": 4, "DeferredRoot": 4, "CleanupRoot": 2, "UnsafeRoot": 3, "SpawnRoot": 1, "MixedRoot": 2, "Service.Check": 3}
			unsupported := 0
			for _, e := range errors {
				if strings.Contains(e.Message, "not implemented yet") {
					unsupported++
					continue
				}
				if e.ID != diagnostics.NoAllocViolation {
					t.Fatalf("unexpected source error: %+v", e)
				}
				cause := e.AllocationCause
				if cause == nil || cause.Incomplete || len(cause.Path) == 0 {
					t.Fatal(e, cause)
				}
				g := a.CallGraph()
				root, _ := g.Node(cause.Path[0])
				length, found := want[root.Name]
				if !found || length != len(cause.Path) {
					t.Fatal(depth, profile, root.Name, cause)
				}
				delete(want, root.Name)
				if root.Name == "MixedRoot" {
					if e.ProofState != diagnostics.ProofInvalid || cause.Unknown {
						t.Fatal(e)
					}
				} else {
					if e.ProofState != diagnostics.ProofUnproven || !cause.Unknown || !strings.Contains(e.Help, "complete allocation-free proof") || strings.Contains(e.Help, "remove @noAlloc") {
						t.Fatal(e)
					}
					terminal := cause.Steps[len(cause.Steps)-1]
					if terminal.Source.File != file || terminal.Source.Line != e.PreviousLine || terminal.Source.Column != e.PreviousColumn || terminal.Message == allocationInvalidSourceReason {
						t.Fatal(e, terminal)
					}
					if root.Name == "CleanupRoot" && !strings.Contains(terminal.Message, "cleanup") {
						t.Fatal(terminal)
					}
					if root.Name == "BoundRoot" && !strings.Contains(terminal.Message, "closure") {
						t.Fatal(terminal)
					}
					if root.Name == "SpawnRoot" && len(cause.Path) != 1 {
						t.Fatal("worker context leaked", cause)
					}
				}
			}
			if len(want) != 0 || unsupported != 11 {
				t.Fatal(depth, profile, want, unsupported, errors)
			}
			// A negative policy consumer must never strengthen the public facts.
			for _, fact := range a.AllocationFacts() {
				if fact.Knowledge == AllocationFree {
					t.Fatal("invalid ISR source acquired positive proof", fact)
				}
			}
			if again := a.Analyze(program); !reflect.DeepEqual(errors, again) {
				t.Fatal("reanalysis changed diagnostics", errors, again)
			}
		}
	}
}
