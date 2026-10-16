package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestDynamicArrayMutationBorrows checks structural invalidation against live
// element/slice dependencies, aliases, field projections and deferred uses at
// every depth, while preserving ended loans and disjoint element/field writes.
// Rules: rules/collections/collections.md — §§6.5,6.7–6.9,19;
// rules/memory/borrowing.md — §§8.1,12,14.5,21.
func TestDynamicArrayMutationBorrows(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, invalid := range []bool{false, true} {
			suffix, want := "valid", 0
			if invalid {
				suffix, want = "invalid", 10
			}
			path := "../../testdata/sema/dynamic_array_borrows_" + suffix + ".sec"
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), path))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			errs := a.Analyze(program)
			if len(errs) != want {
				t.Fatalf("depth %v %s: %+v", depth, suffix, errs)
			}
			if repeated := a.Analyze(program); len(repeated) != want {
				t.Fatalf("reanalysis %v %s: %+v", depth, suffix, repeated)
			}
			if !invalid {
				continue
			}
			for _, diagnostic := range errs {
				if (diagnostic.ID != diagnostics.BorrowMutationConflict && diagnostic.ID != diagnostics.BorrowOwnerReadConflict) || diagnostic.PreviousLine == 0 || diagnostic.Help == "" {
					t.Fatal(diagnostic)
				}
			}
			if len(a.resolvedRangeAppends) != 0 {
				t.Fatal("rejected range Append published a lowering plan")
			}
			for _, fact := range a.AllocationContextFacts() {
				if fact.Kind == ArenaEffectCollectionGrowth {
					t.Fatal("rejected mutation published growth", fact)
				}
			}
		}
	}
}
