package main

import (
	"strings"
	"testing"
)

// TestArenaMultiOriginCLI verifies the analysis tool and backend entry gate
// expose every joined-domain validity failure with its causal source link.
// Rules: rules/memory/lifetime_analysis.md — §11.2;
// rules/tooling/diagnostics.md — §§9,14.
func TestArenaMultiOriginCLI(t *testing.T) {
	path := "../../testdata/sema/arena_multi_origin_invalid.sec"
	for _, command := range []string{"check", "analyse", "emit-ir"} {
		args := []string{command, path, "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", "-")
		}
		_, stderr, code := runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(command, code, stderr)
		}
		doc := decodeOccurrenceDocument(t, stderr)
		if doc.Summary.Errors != 9 || len(doc.Occurrences) != 9 {
			t.Fatal(command, doc)
		}
		for _, value := range doc.Occurrences {
			if value.Primary == nil || value.Primary.Span.File != path || len(value.Related) != 1 || value.Related[0].Span.File != path {
				t.Fatal(value)
			}
		}
		if !strings.Contains(stderr, "after arena second was reset") {
			t.Fatal(stderr)
		}
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/arena_multi_origin_valid.sec", "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, stderr)
	}
}
