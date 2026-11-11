package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Canonical yield facts distinguish logical scheduling/cancellation and physical
// hints, retain source provenance, and cannot be mutated through snapshots.
// Rules: rules/concurrency/scheduling.md §§7–15,50,55–56.
func TestResolvedSchedulingPoints(t *testing.T) {
	parse := func(name string) *ast.Program {
		t.Helper()
		file := "../../testdata/sema/" + name
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(source), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		return program
	}
	program := parse("scheduling_point_facts_valid.sec")
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	facts := a.SchedulingPoints()
	if len(facts) != 3 {
		t.Fatal(facts)
	}
	for i, fact := range facts {
		task := i == 0
		if fact.Source.File != "../../testdata/sema/scheduling_point_facts_valid.sec" || fact.Source.Lexeme != "Yield" || fact.TaskSchedulingPoint != task || fact.CurrentCancellationPoint != task || fact.PhysicalSchedulerHint == task {
			t.Fatal(fact)
		}
	}
	matched := 0
	if err := walk.Inspect(program, func(node any) error {
		if call, ok := node.(*ast.CallExpression); ok {
			fact, found := a.SchedulingPointOf(call)
			if found {
				matched++
				if fact.MemberID != "CKM-TASK-YIELD" && fact.MemberID != "CKM-THREAD-YIELD" {
					t.Fatal(fact)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if matched != 3 {
		t.Fatal(matched)
	}
	original := a.SchedulingPoints()
	facts[0].MemberID = "corrupted"
	facts[0].CurrentCancellationPoint = false
	if !reflect.DeepEqual(original, a.SchedulingPoints()) {
		t.Fatal("snapshot mutated producer")
	}
	if errors := a.Analyze(program); len(errors) != 0 || !reflect.DeepEqual(original, a.SchedulingPoints()) {
		t.Fatal("re-analysis changed facts", errors)
	}
	// Invalid signatures and instance calls must not acquire concurrency facts.
	// The direct Task.Yield in main has a valid signature but separately fails
	// its execution-context proof; retaining its fact must not assert validity.
	invalid := parse("yield_surface_invalid.sec")
	if errors := a.Analyze(invalid); len(errors) != 7 {
		t.Fatal(errors)
	}
	if facts := a.SchedulingPoints(); len(facts) != 1 || facts[0].MemberID != "CKM-TASK-YIELD" {
		t.Fatal(facts)
	}
	if errors := a.Analyze(parse("thread_legacy_errors_local.sec")); len(errors) != 0 || len(a.SchedulingPoints()) != 0 {
		t.Fatal("facts leaked into another program", errors, a.SchedulingPoints())
	}
	if _, ok := a.SchedulingPointOf(nil); ok {
		t.Fatal("nil call fact")
	}
	var empty *Analyzer
	if len(empty.SchedulingPoints()) != 0 {
		t.Fatal("nil analyzer snapshot")
	}
	if _, ok := empty.SchedulingPointOf(nil); ok {
		t.Fatal("nil analyzer fact")
	}
}
