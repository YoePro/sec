package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// LSP publishes the same mandatory missing-relation errors as compiler check,
// while retaining legal string and checked integer enum conversion behavior.
// Rules: rules/types/types.md — Explicit conversions (MD-012 §4.1);
// rules/declarations/enums.md — §12; rules/tooling/diagnostics.md — §§14–15.
func TestEnumCheckedAnalysisRelationsLSP(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_conversion_relations/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := 0
		for _, d := range analyze("file:///tmp/sec-enum-relations/main.sec", string(data)) {
			if d.Severity != 1 {
				continue
			}
			errors++
			if name == "invalid" && (d.Code != diagnostics.ConversionUnavailable || !strings.Contains(d.Message, "help:") || d.Range.End.Character <= d.Range.Start.Character) {
				t.Fatal(d)
			}
		}
		if name == "valid" && errors != 0 || name == "invalid" && errors != 6 {
			t.Fatalf("%s: errors=%d", name, errors)
		}
	}
}
