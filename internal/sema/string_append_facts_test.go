package sema

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Append plans retain maximal RHS segments and detached protected failure
// lists, and every append allocation site has its resolved ambient context.
// Rules: rules/foundations/operators.md — §§29.3,29.9;
// rules/memory/allocation.md — §§17,25; rules/errors/errorhandling.md — §35.
func TestMD039StringAppendFacts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/md039/append.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "append.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	fused, failures := false, false
	for value, stored := range analyzer.stringAppends {
		if len(stored.Plan.Segments) == 0 {
			t.Fatal("missing canonical RHS segments")
		}
		plan, found := analyzer.ResolvedStringAppendOf(value)
		if !found {
			t.Fatal("missing append plan")
		}
		if len(plan.Plan.Segments) > 1 {
			fused = true
		}
		plan.Plan.Segments[0].Kind = "invalid"
		again, _ := analyzer.ResolvedStringAppendOf(value)
		if again.Plan.Segments[0].Kind == "invalid" {
			t.Fatal("append plan aliases its consumer")
		}
	}
	for stmt, stored := range analyzer.resolvedTryAssignments {
		if len(stored.Failures) == 0 {
			continue
		}
		failures = true
		fact, _ := analyzer.ResolvedTryAssignmentOf(stmt)
		fact.Failures[0].Kind = "invalid"
		again, _ := analyzer.ResolvedTryAssignmentOf(stmt)
		if again.Failures[0].Kind == "invalid" {
			t.Fatal("protected RHS failures alias their consumer")
		}
	}
	sites := 0
	for _, fact := range analyzer.AllocationContextFacts() {
		if fact.Kind != ArenaEffectStringMaterialization {
			continue
		}
		sites++
		if !fact.ContextKnown || !fact.Context.Available || !analyzer.IsLowerableStringMaterialization(fact) {
			t.Fatal("lost append allocation context", fact)
		}
	}
	if !fused || !failures || sites < len(analyzer.stringAppends) {
		t.Fatal("missing append/failure/allocation facts", fused, failures, sites)
	}
}
