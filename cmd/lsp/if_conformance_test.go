package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestIfConformanceTruthinessLSP preserves the compiler's bool-only contract
// and focused nonempty diagnostics for every non-boolean condition family in
// an unsaved source snapshot, including references and raw pointers.
// Rules: rules/control-flow/flowcontrol_if.md — §§3,27–28;
// rules/tooling/lsp.md — Diagnostics.
func TestIfConformanceTruthinessLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/control_flow/if_conformance/truthiness_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	items := analyze(uri, string(source))
	if len(items) != 9 {
		t.Fatal(items)
	}
	for i, d := range items {
		if d.Code != diagnostics.IfConditionNotBool || d.Severity != 1 || d.Range.Start.Line != 9+i || comparePosition(d.Range.Start, d.Range.End) >= 0 || !strings.Contains(d.Message, "if condition must be bool") || !strings.Contains(d.Message, "explicit comparison") {
			t.Fatalf("incomplete or misplaced condition diagnostic: %+v", d)
		}
	}
}
