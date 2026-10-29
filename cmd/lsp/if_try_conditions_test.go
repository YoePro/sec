package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIfTryConditionsLSP checks unsaved-source diagnostics at each try token
// and the same bool-only success contract as compiler/Sema analysis.
// Rules: rules/control-flow/flowcontrol_if.md — §§11,27–28;
// rules/tooling/lsp.md — Diagnostics.
func TestIfTryConditionsLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"valid", "non_bool_invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/if_try/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(source))
		want := 0
		if name != "valid" {
			want = 4
		}
		if len(items) != want {
			t.Fatal(name, items)
		}
		for i, item := range items {
			if item.Severity != 1 || item.Range.Start.Line != 8+i || item.Range.End.Character-item.Range.Start.Character != 3 || !strings.Contains(item.Message, "if condition") || !strings.Contains(item.Message, "bool") {
				t.Fatal(item)
			}
		}
	}
}
