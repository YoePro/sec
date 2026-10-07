package sema

import (
	"os"
	"reflect"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestAllocationFacts verifies proof versus incomplete evidence, independent
// effects, synchronous routes, recursion and trusted foreign contracts.
// Rules: rules/memory/allocation.md — §§4,6(4),24,29(1)-(2),30.
func TestAllocationFacts(t *testing.T) {
	file := "../../testdata/sema/allocation_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			graph := a.CallGraph()
			facts := a.AllocationFacts()
			baseline := a.AllocationFacts()
			if !reflect.DeepEqual(facts, a.AllocationFacts()) {
				t.Fatal("facts are not deterministic")
			}
			expected := map[string]AllocationKnowledge{
				"Free": AllocationFree, "FreeForward": AllocationFree, "Trusted": AllocationFree,
				"TrustedCaller": AllocationFree, "Recursive": AllocationFree, "main": AllocationFree,
				"Allocate": AllocationMayAllocate, "Forward": AllocationMayAllocate, "Mixed": AllocationMayAllocate,
				"Foreign": AllocationUnknown, "Unresolved": AllocationUnknown, "RecursiveUnknown": AllocationUnknown,
				"Spawner": AllocationUnknown, "Open": AllocationUnknown,
				"Creator": AllocationUnknown, "lambda": AllocationUnknown,
			}
			for _, fact := range facts {
				node, _ := graph.Node(fact.Callable)
				want, ok := expected[node.Name]
				if !ok {
					t.Fatalf("unexpected node %s", node.Name)
				}
				if fact.Knowledge != want {
					t.Fatalf("%s: got %+v want %s", node.Name, fact, want)
				}
				if !reflect.DeepEqual(fact, a.AllocationFact(fact.Callable)) {
					t.Fatal("single and batch facts differ")
				}
				if fact.Knowledge == AllocationFree && (fact.HasUnknown || len(fact.AllocationPath) > 0) {
					t.Fatal(fact)
				}
				if fact.Knowledge == AllocationUnknown && (!fact.HasUnknown || len(fact.UnknownEvidence) == 0) {
					t.Fatal(fact)
				}
				if node.Name == "Mixed" && !fact.HasUnknown {
					t.Fatal("mixed effects lost unknown evidence")
				}
				if node.Name == "Forward" {
					names := []string{}
					for _, id := range fact.AllocationPath {
						n, _ := graph.Node(id)
						names = append(names, n.Name)
					}
					if strings.Join(names, " -> ") != "Forward -> Allocate" {
						t.Fatal(names)
					}
				}
				if node.Name == "Spawner" && len(fact.AllocationPath) > 0 {
					t.Fatal("spawn execution leaked into synchronous route")
				}
				if len(fact.AllocationPath) > 0 {
					fact.AllocationPath[0] = "corrupted"
				}
				if len(fact.UnknownPath) > 0 {
					fact.UnknownPath[0] = "corrupted"
				}
				if len(fact.UnknownEvidence) > 0 {
					fact.UnknownEvidence[0].Reason = "corrupted"
				}
			}
			if len(facts) != len(expected) || !reflect.DeepEqual(baseline, a.AllocationFacts()) {
				t.Fatal("coverage or detached result failure")
			}
			missing := a.AllocationFact("missing")
			if missing.Knowledge != AllocationUnknown || !missing.HasUnknown {
				t.Fatal(missing)
			}
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if !reflect.DeepEqual(baseline, a.AllocationFacts()) {
				t.Fatal("reanalyzing changed facts")
			}
			// Reusing an analyzer must remove facts for bodies no longer in the module.
			empty := parser.New(lexer.NewWithFile("module main\n", file)).ParseProgram()
			a.Analyze(empty)
			if len(a.AllocationFacts()) != 0 {
				t.Fatal("stale callable facts")
			}
		})
	}
}

// TestAllocationFactsRejectInvalidFreedom keeps erroneous source from acquiring
// an allocation-free proof while preserving the original semantic errors.
// Rules: rules/memory/allocation.md — §§24(6),29(2).
func TestAllocationFactsRejectInvalidFreedom(t *testing.T) {
	file := "../../testdata/sema/pitfall_diagnostic_ownership_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errs := a.Analyze(program)
	if len(errs) == 0 {
		t.Fatal("expected invalid fixture")
	}
	facts := a.AllocationFacts()
	if len(facts) == 0 {
		t.Fatal("missing represented callables")
	}
	for _, fact := range facts {
		if fact.Knowledge == AllocationFree || !fact.HasUnknown {
			t.Fatal(fact)
		}
	}
	if !reflect.DeepEqual(errs, a.errors) {
		t.Fatal("allocation view changed diagnostics")
	}
}
