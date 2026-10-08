package collectionshape_test

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// TestEmptyListLengthContracts covers typed storage, inherited and capacity
// types, conversions, fields, arguments and returns with defining provenance.
// Rules: rules/types/contracts.md — "Composition", "String and collection
// contracts", "Initialization and assignment", "Diagnostics";
// rules/collections/collections.md — §13.3.
func TestEmptyListLengthContracts(t *testing.T) {
	errors := analyzeSource(t, fixture(t, "empty_lists_invalid.sec"))
	if len(errors) != 9 {
		t.Fatalf("errors = %v, want 9", errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.ValueViolatesContract || !strings.Contains(err.Message, "list literal length 0 violates") {
			t.Errorf("unrelated diagnostic: %+v", err)
		}
		if err.PreviousLine == 0 {
			t.Errorf("missing defining contract: %+v", err)
		}
	}
}

// TestListDefaultsRespectContracts checks nested aggregate defaultability and
// previews independently of the element default's existence or list capacity.
// Rules: rules/types/default_values.md — "Defaults and contracts", "Struct
// defaults", "Arrays", "List defaults".
func TestListDefaultsRespectContracts(t *testing.T) {
	a, errors := analyzeSourceWithAnalyzer(t, fixture(t, "list_defaults_invalid.sec"))
	if len(errors) != 9 {
		t.Fatalf("errors = %v, want 9", errors)
	}
	for _, name := range []string{"Positive", "ExactlyOne", "Present", "Inherited", "Bounded", "Holder", "Nested"} {
		typ := a.Types()[name]
		if sema.IsDefaultable(typ) {
			t.Errorf("%s retained invalid default", name)
		}
		if text, kind, ok := sema.DefaultValuePreview(typ, 8); ok || kind != sema.NoDefault || text != "" {
			t.Errorf("%s preview = %q %s %v", name, text, kind, ok)
		}
	}
}

// TestEmptyListContractsAcceptValidShapes preserves allocation-free defaults
// without requiring any element default, even through aggregates and arrays.
// Rules: rules/types/default_values.md — "List defaults", "Struct defaults";
// rules/types/contracts.md — "String and collection contracts".
func TestEmptyListContractsAcceptValidShapes(t *testing.T) {
	a, errors := analyzeSourceWithAnalyzer(t, fixture(t, "empty_lists_valid.sec"))
	assertSemaErrors(t, errors, nil)
	for _, name := range []string{"Empty", "Loose", "Unique", "Bounded", "Inherited", "Holder", "Nested", "Zero"} {
		if !sema.IsDefaultable(a.Types()[name]) {
			t.Errorf("%s lost valid default", name)
		}
	}
	if sema.IsDefaultable(a.Types()["Handle"]) {
		t.Fatal("test requires nondefaultable element")
	}
	if sema.IsDefaultable(a.Types()["Required"]) {
		t.Fatal("zero-array control requires a nondefaultable list element")
	}
}
