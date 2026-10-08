package collectionshape_test

import (
	"sec/internal/sema"
	"testing"
)

// Empty list literals type as their list type, and list[T] and
// list[T, Capacity] are defaultable independently of element defaultability,
// both for mutable declarations and omitted struct fields.
//
// Rules:
//   - rules/types/default_values.md — "List defaults"
//   - rules/collections/collections.md — §13.3 empty list state
func TestEmptyListLiteralsAndListDefaults(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, fixture(t, "collection_literal_1.sec"))
	if len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	for _, name := range []string{"Holder"} {
		if value, kind, ok := sema.DefaultValueDisplay(analyzer.Types()[name]); !ok || kind != sema.StructDefault {
			t.Fatalf("%s default = %q %q %v", name, value, kind, ok)
		}
	}
}

// A capacity mismatch and a zero capacity are rejected; a list default
// displays as its canonical empty literal.
//
// Rules:
//   - rules/collections/collections.md — §13.3 and list capacity
//   - rules/types/default_values.md — "List defaults"
func TestEmptyListLiteralCapacityChecks(t *testing.T) {
	errors := analyzeSourceRaw(t, fixture(t, "collection_literal_2.sec"))
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want capacity mismatch and zero capacity", errors)
	}
	if value, kind, ok := sema.DefaultValueDisplay(sema.Type{Name: "list", Kind: sema.StructType, TypeArgs: []sema.Type{{Name: "int", Kind: sema.IntType}}}); !ok || kind != sema.CollectionDefault || value != "list[int] {}" {
		t.Fatalf("list default display = %q %q %v", value, kind, ok)
	}
}
