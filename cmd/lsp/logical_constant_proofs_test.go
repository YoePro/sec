package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestSharedConstantConditionLSP transports the canonical Sema branch proof
// without a parallel editor evaluator and retains unsaved-source locations.
// Rules: rules/control-flow/flowcontrol_if.md — §20;
// rules/control-flow/flowcontrol_while.md — §19; rules/tooling/lsp.md — "Shared diagnostic model".
func TestSharedConstantConditionLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"valid", "unreachable_invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/logical_constants/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(source))
		if name == "valid" {
			for _, d := range items {
				if d.Severity == 1 {
					t.Fatal(d)
				}
			}
			continue
		}
		if len(items) != 2 {
			t.Fatal(items)
		}
		for i, line := range []int{5, 8} {
			d := items[i]
			if d.Code != diagnostics.UnreachableStatement || d.Severity != 1 || d.Range.Start.Line != line-1 || !strings.Contains(d.Message, "help:") {
				t.Fatal(d)
			}
		}
	}
}
