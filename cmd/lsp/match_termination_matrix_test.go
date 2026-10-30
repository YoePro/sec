package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestMatchTerminationLSP preserves the compiler's normalized return and
// unreachable proofs in unsaved source without an editor-owned flow model.
// Rules: rules/control-flow/flowcontrol_match.md — §23;
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestMatchTerminationLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"valid", "continuing_invalid", "unreachable_invalid"} {
		data, err := os.ReadFile("../../testdata/control_flow/match_termination/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := analyze(uri, string(data))
		if name == "valid" {
			for _, d := range errors {
				if d.Severity == 1 {
					t.Fatal(d)
				}
			}
			continue
		}
		if len(errors) != 6 {
			t.Fatal(name, errors)
		}
		for _, d := range errors {
			if d.Severity != 1 || d.Range.Start.Line == 0 {
				t.Fatal(d)
			}
			if name == "unreachable_invalid" && (d.Code != diagnostics.UnreachableStatement || !strings.Contains(d.Message, "help:")) {
				t.Fatal(d)
			}
		}
	}
}
