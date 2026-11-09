package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// Null facts survive immutable pointer copies and chains, including copies on
// stable proven-null paths. Copies of unknown/reassigned values stay unknown.
// Rules: rules/memory/raw_pointers.md §§5.1,7(6–8),32,34(2–4).
func TestRawNullAliases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		errors int
	}{
		{"raw_null_alias_valid.sec", 0}, {"raw_null_alias_invalid.sec", 8},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.name)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatalf("%s: %v", tc.name, errors)
		}
		for _, e := range errors {
			if e.ID != diagnostics.RawPointerNullDereference {
				t.Fatal(e)
			}
		}
		if tc.errors != 0 {
			graph := a.CallGraph()
			for _, name := range []string{"CopyRead", "CopyWrite", "ChainedVolatileRead", "CopyVolatileWrite", "CopyAsRef", "CopyAsRefMut", "PathCopy", "FalseEdgeCopy"} {
				id := callGraphNodeIDByName(t, graph, name)
				if effects := graph.EffectSummary(id).DirectEffects; len(effects) != 0 {
					t.Fatalf("%s published rejected access: %+v", name, effects)
				}
			}
		}
	}
}
