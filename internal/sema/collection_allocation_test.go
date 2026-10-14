package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCollectionGrowthAllocation checks canonical growth, source provenance and
// transitive guarantees at every depth, with bounded/empty/user-method controls.
// Rules: rules/memory/allocation.md — §§3,24,29;
// rules/collections/collections.md — §§6.7,13.2,13.10,13.12,14.8,15.5.
func TestCollectionGrowthAllocation(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		suffix := "valid"
		want := 0
		if invalid {
			suffix = "invalid"
			want = 11
		}
		path := "../../testdata/sema/collection_growth_" + suffix + ".sec"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(data), path))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			errors := a.Analyze(program)
			if len(errors) != want {
				t.Fatalf("%s/%s: %v; want %d errors", suffix, depth, errors, want)
			}
			for _, e := range errors {
				if e.ID != diagnostics.NoAllocViolation || len(e.RelatedLocations()) == 0 || e.RelatedLocations()[0].File != path || e.RelatedLocations()[0].Line == 0 {
					t.Fatalf("missing owning diagnostic/provenance: %+v", e)
				}
			}
			graph := a.CallGraph()
			for _, node := range graph.Nodes() {
				summary := graph.ArenaSummary(node.ID)
				if summary.MayAllocate != invalid {
					t.Fatalf("%s: %+v", node.Name, summary)
				}
				if invalid && node.Name != "Forward" {
					if len(summary.DirectEffects) != 1 || summary.DirectEffects[0].Kind != ArenaEffectCollectionGrowth || summary.DirectEffects[0].Domain != "" {
						t.Fatalf("incorrect canonical growth effect: %s %+v", node.Name, summary)
					}
				}
			}
		}
	}
}
