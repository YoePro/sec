package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestComputedStringSwitchLSP consumes Sema's duplicate proofs in unsaved
// source and retains the primary span, help and navigable earlier-case link.
// Rules: rules/control-flow/flowcontrol_switch.md — §§20,25;
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestComputedStringSwitchLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"valid", "duplicate_invalid"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/switch_string_constants/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			got := analyze(uri, string(source))
			if name == "valid" {
				for _, d := range got {
					if d.Severity == 1 {
						t.Fatal(d)
					}
				}
				return
			}
			if len(got) != 3 {
				t.Fatal(got)
			}
			for i, lines := range [][2]int{{6, 5}, {11, 11}, {17, 16}} {
				d := got[i]
				if d.Code != diagnostics.DuplicateSwitchCase || d.Severity != 1 || d.Range.Start.Line != lines[0]-1 || !strings.Contains(d.Message, "help:") || len(d.RelatedInformation) != 1 {
					t.Fatalf("incorrect duplicate diagnostic: %+v", d)
				}
				link := d.RelatedInformation[0]
				if link.Location.URI != uri || link.Location.Range.Start.Line != lines[1]-1 || link.Message != "earlier equivalent string case" {
					t.Fatalf("incorrect duplicate provenance: %+v", d)
				}
			}
		})
	}
}
