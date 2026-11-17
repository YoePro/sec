package sema

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Known storage ends and live alias conflicts also constrain raw accesses;
// unsafe accepts unknown obligations but cannot contradict canonical facts.
// Rules: rules/memory/raw_pointers.md §§11–13,23(4),24(7),34.
func TestRawOriginAccess(t *testing.T) {
	for _, kind := range []string{"invalid", "valid"} {
		data, err := os.ReadFile("../../testdata/sema/raw_origin_access_" + kind + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		want := 0
		if kind == "invalid" {
			want = 7
		}
		if len(errors) != want {
			t.Fatalf("%s: %v, want %d errors", kind, errors, want)
		}
		for i, e := range errors {
			expected := diagnostics.RawPointerDanglingOrigin
			if i == 5 {
				expected = diagnostics.BorrowOwnerReadConflict
			}
			if i == 6 {
				expected = diagnostics.BorrowMutationConflict
			}
			if e.ID != expected {
				t.Fatalf("unexpected error: %v", e)
			}
		}
		if kind == "invalid" {
			graph := a.CallGraph()
			for _, name := range []string{"DeadRead", "DeadWrite", "DeadVolatileRead", "DeadVolatileWrite", "MovedFieldRead"} {
				for _, effect := range graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).DirectEffects {
					if strings.Contains(string(effect.Kind), "memory") || strings.Contains(string(effect.Kind), "volatile") {
						t.Fatalf("rejected access published effect: %+v", effect)
					}
				}
			}
		}
	}
}
