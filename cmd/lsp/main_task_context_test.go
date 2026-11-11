package main

import (
	"os"
	"strings"
	"testing"
)

// TestMainTaskContextLSP projects the compiler's ordinary-main rejection and
// navigable witnesses, while preserving actual task and physical-thread roots.
// Rules: rules/concurrency/scheduling.md — §§21–22,55,57;
// rules/tooling/diagnostics.md — §§8–12.
func TestMainTaskContextLSP(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/main_task_context_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		count, related := 0, 0
		operations := map[string]bool{}
		for _, d := range analyze("file:///main-task-context/main.sec", string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1213" || !strings.Contains(d.Message, "ordinary program entry is not a logical task") {
				t.Fatal(d)
			}
			for _, name := range []string{"Task.Current()", "Task.Yield()"} {
				if strings.Contains(d.Message, name) {
					operations[name] = true
				}
			}
			if strings.Contains(d.Message, "because") {
				related++
				if len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Message != "current-execution requirement introduced here" {
					t.Fatal(d)
				}
			}
		}
		if mode == "valid" && count != 0 || mode == "invalid" && (count != 6 || related != 4 || len(operations) != 2) {
			t.Fatalf("%s: errors=%d related=%d operations=%v", mode, count, related, operations)
		}
	}
}
