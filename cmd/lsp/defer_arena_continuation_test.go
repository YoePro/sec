package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDeferredArenaContinuationLSP consumes Sema's separation of registered
// cleanup from ordinary continuation, retaining actual stale-use/source links.
// Rules: rules/control-flow/defer.md — §§3,9,29;
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestDeferredArenaContinuationLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"valid", "immediate_invalid"} {
		data, err := os.ReadFile("../../testdata/control_flow/defer_arena_continuation/" + name + ".sec")
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
		if len(errors) != 3 {
			t.Fatal(errors)
		}
		for i, lines := range [][2]int{{7, 5}, {13, 11}, {20, 17}} {
			d := errors[i]
			if d.Severity != 1 || d.Range.Start.Line != lines[0]-1 || !strings.Contains(d.Message, "cannot use values after arena mem") || len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.URI != uri || d.RelatedInformation[0].Location.Range.Start.Line != lines[1]-1 {
				t.Fatal(d)
			}
		}
	}
}
