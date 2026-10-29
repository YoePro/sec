package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestStringEnumCTECLI checks that composed constant members pass frontend
// checking and unavailable CTE execution uses the canonical diagnostic contract.
// Rules: rules/declarations/enums.md — §5;
// rules/compiler/compile_time_evaluation.md — §8(5).
func TestStringEnumCTECLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/string_enum_cte/valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/string_enum_cte/invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 {
		t.Fatal(code, stderr)
	}
	found := false
	for _, d := range doc.Occurrences {
		if d.ID != nil && *d.ID == diagnostics.SemanticCompileTimeExecutionUnavailable {
			found = true
			if d.Unregistered || d.Primary == nil || len(d.Help) == 0 || !strings.Contains(d.Message.Text, "string-backed enum member NeedsExecution.Value initializer") {
				t.Fatal(d)
			}
		}
	}
	if !found {
		t.Fatal("missing canonical unavailable-CTE occurrence", stderr)
	}
}
