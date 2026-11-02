package sema

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Missing conversion relations are compile-time errors, never runtime
// ConversionError results or silently invalid expression types.
// Rules: rules/declarations/enums.md — §§5,12.1–12.3,12.6;
// rules/types/types.md — "Explicit conversions" (MD-012 §4.1).
func TestEnumCheckedAnalysisRejectsMissingRelations(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_relations/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 6 {
		t.Fatalf("errors: %v", errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.ConversionUnavailable || err.Help == "" || !strings.Contains(err.Message, "convert") {
			t.Fatalf("missing canonical no-relation diagnostic: %+v", err)
		}
	}
}

// Integer enums retain exact/checked/known-value analysis, while string enums
// retain their distinct legal string conversion channel.
// Rules: rules/declarations/enums.md — §12; rules/types/types.md — Explicit conversions.
func TestEnumCheckedAnalysisRetainsLegalRelations(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_relations/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(data)); len(errors) != 0 {
		t.Fatal(errors)
	}
}
