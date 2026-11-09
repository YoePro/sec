package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestNonBindingMatchPayloadsRejectLifecycleObligations checks that every
// non-binding variant form preserves recursive lifecycle obligations.
// Rules: rules/control-flow/flowcontrol_match.md §8(5);
// rules/control-flow/discard.md §18(1)-(3).
func TestNonBindingMatchPayloadsRejectLifecycleObligations(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/match_payload_discards_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 17 {
		t.Fatalf("want 17 discard errors, got %d: %+v", len(errors), errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.NonDiscardableValue || !strings.Contains(err.Message, "cannot ignore") || err.Help == "" || err.Line == 0 {
			t.Errorf("expected source-positioned S1006 with help, got %+v", err)
		}
	}
	if len(analyzer.resolvedMatchPlans) != 0 {
		t.Fatalf("invalid match published a plan: %+v", analyzer.resolvedMatchPlans)
	}
}

// TestNonBindingMatchPayloadsKeepDiscardableForms accepts ordinary payloads,
// non-owning reference payloads and bindings that retain lifecycle authority.
// Rules: rules/control-flow/flowcontrol_match.md §§8(5),13-15;
// rules/control-flow/discard.md §§18,20.
func TestNonBindingMatchPayloadsKeepDiscardableForms(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/match_payload_discards_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 0 {
		t.Fatalf("valid payload patterns rejected: %+v", errors)
	}
	if len(analyzer.resolvedMatchPlans) != 8 {
		t.Fatalf("want 8 resolved match plans, got %d", len(analyzer.resolvedMatchPlans))
	}
}
