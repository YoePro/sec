package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The editor must present the same copyability failure as the compiler, with
// a stable ID, source span and concrete explanation for empty and nonempty forms.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §7.9; rules/tooling/lsp.md — Diagnostics.
func TestAnalyzeFillCopyabilityDiagnostic(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/fill_copyability_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	items := analyze(uri, string(data))
	if len(items) != 5 {
		t.Fatalf("diagnostics = %+v, want five copyability errors", items)
	}
	for _, item := range items {
		if item.Code != "S1156" || item.Range.Start.Line == 0 || !strings.Contains(item.Message, "copyable element type") || !strings.Contains(item.Message, "evaluates its source once") {
			t.Fatalf("incomplete editor diagnostic: %+v", item)
		}
	}
}
