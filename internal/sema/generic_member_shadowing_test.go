package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// TestGenericMemberShadowing rejects outer generic/member collisions while
// accepting sibling method scopes and ordinary re-entry of an owner's parameters.
// Rules: rules/foundations/names_scopes_visibility.md — §§8,15;
// rules/declarations/generics.md — §9.
func TestGenericMemberShadowing(t *testing.T) {
	for _, tc := range []struct {
		file  string
		count int
	}{{"valid.sec", 0}, {"invalid.sec", 8}} {
		data, err := os.ReadFile("../../testdata/names/generic_member_shadowing/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzer(t, string(data))
		if len(errors) != tc.count {
			t.Fatalf("%s: %v; want %d errors", tc.file, errors, tc.count)
		}
		origins := map[int]int{8: 7, 9: 4, 10: 14, 11: 17, 12: 15, 13: 16, 21: 20, 22: 4}
		for _, issue := range errors {
			if expected := origins[issue.Line]; expected == 0 || issue.PreviousLine != expected {
				t.Fatalf("wrong hidden declaration: %+v", issue)
			}
			if issue.ID != diagnostics.GenericParameterShadowsDeclaration || issue.PreviousLine == 0 {
				t.Fatalf("missing identity/origin: %+v", issue)
			}
		}
	}
}

// TestGenericMemberShadowingBeforeBodyExpansion keeps a later member's
// declaration visible even when the pack body cannot yet be expanded.
// Rules: rules/foundations/names_scopes_visibility.md — §§8,15;
// rules/declarations/generics.md — §§9,16A.
func TestGenericMemberShadowingBeforeBodyExpansion(t *testing.T) {
	data, err := os.ReadFile("../../testdata/names/generic_member_shadowing/unsupported_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, issues := analyzeSourceWithAnalyzer(t, string(data))
	shadow, capability := 0, 0
	for _, issue := range issues {
		switch issue.ID {
		case diagnostics.GenericParameterShadowsDeclaration:
			shadow++
			if issue.PreviousLine != 8 {
				t.Fatal(issue)
			}
		case diagnostics.GenericTypePackExpansionUnavailable:
			capability++
		default:
			t.Fatal(issue)
		}
	}
	if shadow != 1 || capability != 1 {
		t.Fatalf("missing header/capability facts: %v", issues)
	}
}
