package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// TestCoveredSwitchItemsLSP keeps coverage proof locations navigable in an
// unsaved document without an editor-owned coverage checker.
// Rules: rules/control-flow/flowcontrol_switch.md §34;
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestCoveredSwitchItemsLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/switch_covered_items_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	got := analyze(uriFromPath(filepath.Join(t.TempDir(), "main.sec")), string(source))
	count := 0
	for _, d := range got {
		if d.Severity != 1 {
			continue
		}
		count++
		if (d.Code != diagnostics.UnreachableStatement && d.Code != diagnostics.DuplicateSwitchCase) || len(d.RelatedInformation) != 1 || d.Range.End.Character <= d.Range.Start.Character || d.RelatedInformation[0].Message == "" {
			t.Errorf("incomplete covered-case diagnostic: %+v", d)
		}
	}
	if count != 10 {
		t.Fatalf("want 10 mandatory coverage errors, got %+v", got)
	}
}
