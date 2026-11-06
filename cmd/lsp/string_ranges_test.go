package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Editor and compiler share range typing and mandatory diagnostics with help.
// Rules: rules/collections/collections.md — §7.10; rules/control-flow/flowcontrol_for.md — §19;
// rules/tooling/lsp.md — "Shared diagnostic model" (MD-126).
func TestStringRangesLSP(t *testing.T) {
	for _, name := range []string{"valid", "range_invalid"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/string_ranges/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			count := 0
			for _, d := range analyze(uri, string(source)) {
				if d.Severity != 1 {
					continue
				}
				count++
				if name == "valid" {
					t.Fatal(d)
				}
				if d.Code != diagnostics.InvalidSliceRange && d.Code != diagnostics.InvalidRangeSource {
					t.Fatalf("anonymous/unexpected diagnostic: %+v", d)
				}
				if !strings.Contains(d.Message, "help:") {
					t.Fatalf("missing explanation: %+v", d)
				}
			}
			want := 0
			if name == "range_invalid" {
				want = 6
			}
			if count != want {
				t.Fatalf("errors = %d, want %d", count, want)
			}
		})
	}
}

// Hover exposes the view identity and decoded scalar rather than an owning
// string or a reference to a fabricated stored rune.
// Rules: rules/types/types.md — StringView; flowcontrol_for.md — §19;
// rules/tooling/lsp.md — "Hover" (MD-126).
func TestStringRangeHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/string_ranges/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct{ marker, selected, want string }{
		{"let view :=", "view", "StringView"},
		{"RuneValue(ch)", "ch", "rune"},
		{"view.Len", "Len", "StringView.Len"},
	} {
		offset := strings.Index(source, tc.marker) + strings.Index(tc.marker, tc.selected)
		hover, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, tc.want) {
			t.Fatalf("missing %s: %+v", tc.want, hover)
		}
	}
}

// Editor checking shares StringView recovery/propagation and retains precise
// diagnostic identities for escapes, mutation and unprotected panics.
// Rules: rules/errors/errorhandling.md — §§9,21–22,31;
// rules/tooling/lsp.md — "Shared diagnostic model" (MD-126).
func TestStringRangeTryLSP(t *testing.T) {
	for _, tc := range []struct {
		name string
		want map[string]int
	}{
		{"try_valid", map[string]int{}},
		{"try_origin_invalid", map[string]int{diagnostics.EscapeLocalStorage: 4, diagnostics.BorrowMutationConflict: 1}},
		{"try_invalid", map[string]int{diagnostics.NoPanicViolation: 2, diagnostics.TryPropagationIncompatible: 1, diagnostics.InvalidSliceRange: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/string_ranges/" + tc.name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			for _, d := range analyze(uri, string(source)) {
				if d.Severity == 1 {
					tc.want[d.Code]--
				}
			}
			for id, count := range tc.want {
				if count != 0 {
					t.Fatalf("unexpected diagnostic count for %s: %v", id, tc.want)
				}
			}
		})
	}
}
