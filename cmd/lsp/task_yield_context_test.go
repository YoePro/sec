package main

import (
	"os"
	"strings"
	"testing"
)

// TestTaskYieldContextLSP uses compiler root/call-graph diagnostics and related
// operation locations, including calls through local function values and defer.
// Rules: rules/concurrency/scheduling.md — §§7(2),22,50,57;
// rules/tooling/diagnostics.md — §§8–12; rules/tooling/lsp.md — Diagnostics.
func TestTaskYieldContextLSP(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"task_yield_context_valid.sec", 0}, {"task_yield_context_invalid.sec", 7},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		count := 0
		for _, d := range analyze("file:///task-yield-context/main.sec", source) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1213" || !strings.Contains(d.Message, "logical task context") {
				t.Fatal(d)
			}
			if strings.Contains(d.Message, "because") {
				if len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.Range.Start.Line != 3 || d.RelatedInformation[0].Message != "current-execution requirement introduced here" {
					t.Fatal(d)
				}
			} else if len(d.RelatedInformation) != 0 {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
