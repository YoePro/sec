package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// LSP publishes canonical raw-pointer mentor diagnostics and causal locations,
// without a pointer proof model in the editor or an automatic unsafe rewrite.
// Rules: rules/memory/raw_pointers.md §§34–35; rules/tooling/lsp.md — Shared diagnostic model.
func TestRawPointerMentorLSPDiagnostics(t *testing.T) {
	for _, name := range []string{"raw_pointer_mentor_invalid", "raw_pointer_mentor_valid"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/tooling/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			counts := map[string]int{}
			for _, d := range analyze(uri, string(data)) {
				if d.Severity != 1 {
					continue
				}
				if name == "raw_pointer_mentor_valid" {
					t.Fatal(d)
				}
				counts[d.Code]++
				if !strings.Contains(d.Message, "help:") {
					t.Fatalf("lost mentor help: %+v", d)
				}
				if d.Code == diagnostics.RawPointerNullDereference || d.Code == diagnostics.RawPointerDanglingOrigin {
					if len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.URI != uri {
						t.Fatalf("lost causal source: %+v", d)
					}
				}
			}
			if name == "raw_pointer_mentor_invalid" && (len(counts) != 4 || counts[diagnostics.RawPointerUnsafeRequired] != 11 || counts[diagnostics.RawPointerInvalidOperation] != 21 || counts[diagnostics.RawPointerNullDereference] != 6 || counts[diagnostics.RawPointerDanglingOrigin] != 1) {
				t.Fatal(counts)
			}
		})
	}
}
