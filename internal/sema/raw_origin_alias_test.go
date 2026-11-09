package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// Immutable pointer copies preserve known origin availability and borrow checks,
// without keeping the pointee alive or inventing origins for mutable aliases.
// Rules: rules/memory/raw_pointers.md §§4(4),23(5–7),24(3–7),34.
func TestRawOriginAliases(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{
		{"raw_origin_alias_valid.sec", 0}, {"raw_origin_alias_invalid.sec", 4},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.name)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.count {
			t.Fatalf("%s: %v", tc.name, errors)
		}
		counts := map[string]int{}
		for _, e := range errors {
			counts[e.ID]++
		}
		if tc.count > 0 && (counts[diagnostics.RawPointerDanglingOrigin] != 2 || counts[diagnostics.BorrowMutationConflict] != 1 || counts[diagnostics.BorrowOwnerReadConflict] != 1) {
			t.Fatalf("IDs: %v; errors %v", counts, errors)
		}
	}
}
