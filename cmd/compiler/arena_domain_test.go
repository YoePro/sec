package main

import (
	"strings"
	"testing"
)

// TestArenaMovedParameterDomainCLI requires Reset through a moved parameter
// holder to invalidate caller storage in both compilation and analysis.
// Rules: rules/memory/arena.md — §§4.2,44,68(3)-(4).
func TestArenaMovedParameterDomainCLI(t *testing.T) {
	for _, command := range []string{"check", "analyse"} {
		_, stderr, code := runCLIForDiagnostics(t, command, "../../testdata/sema/arena_domain_alias_invalid.sec", "--diagnostic-format=json")
		if code != 3 {
			t.Fatal(command, code, stderr)
		}
		doc := decodeOccurrenceDocument(t, stderr)
		if doc.Summary.Errors != 2 {
			t.Fatal(doc)
		}
		for _, occurrence := range doc.Occurrences {
			if occurrence.Severity == "error" && !strings.Contains(occurrence.Message.Text, "cannot use values after arena") {
				t.Fatal(occurrence)
			}
		}
	}
}
