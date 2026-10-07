package sema

import (
	"os"
	"reflect"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestAllocationCausePaths checks complete deterministic policy witnesses,
// deferred and recursive paths, definite/unknown separation and spawn exclusion.
// Rules: rules/memory/allocation.md — §§24(6),28(4),29(4).
func TestAllocationCausePaths(t *testing.T) {
	file := "../../testdata/sema/allocation_cause_paths_invalid.sec"
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
	if len(errs) != 4 {
		t.Fatal(errs)
	}
	for _, e := range errs {
		cause := e.AllocationCause
		if e.ID != diagnostics.NoAllocViolation || cause == nil || cause.Incomplete || len(cause.Path) == 0 || len(cause.Steps) < 3 || e.RelatedLabel != "allocation effect introduced here" {
			t.Fatal(e, cause)
		}
		if cause.Steps[0].Kind != "root" || cause.Steps[len(cause.Steps)-1].Kind != "operation" {
			t.Fatal(cause)
		}
		terminal := cause.Steps[len(cause.Steps)-1].Source
		if terminal.File != e.PreviousFile || terminal.Line != e.PreviousLine || terminal.Column != e.PreviousColumn {
			t.Fatal("terminal navigation differs from owning diagnostic", e, cause)
		}
		for _, step := range cause.Steps {
			if step.Source.File != file || step.Source.Line <= 0 || step.Source.Column <= 0 {
				t.Fatal(step)
			}
		}
		root, _ := a.CallGraph().Node(cause.Path[0])
		if (root.Name == "UnknownRoot") != cause.Unknown {
			t.Fatal(root, cause)
		}
		if !reflect.DeepEqual(*cause, a.CallGraph().AllocationCause(root.ID)) {
			t.Fatal("owning diagnostic has a different witness")
		}
		detached := a.CallGraph().AllocationCause(root.ID)
		detached.Path[0] = "corrupted"
		detached.Steps[0].Message = "corrupted"
		if !reflect.DeepEqual(*cause, a.CallGraph().AllocationCause(root.ID)) {
			t.Fatal("mutable witness")
		}
	}
	if cause := a.CallGraph().AllocationCause(callGraphNodeIDByName(t, a.CallGraph(), "Spawner")); len(cause.Path) != 0 || len(cause.Steps) != 0 {
		t.Fatal("spawned allocation leaked into policy witness", cause)
	}
}
