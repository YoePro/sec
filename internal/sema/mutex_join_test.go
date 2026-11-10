package sema

import (
	"os"
	"strings"
	"testing"
)

// TestMutexJoinBoundary rejects live guards at potentially waiting joins, while
// selected ready joins share the enclosing select diagnostic without duplicates.
// Rules: rules/concurrency/mutex.md — §41(2–3); threads.md — §35.
func TestMutexJoinBoundary(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"mutex_join_valid.sec", 0}, {"mutex_join_invalid.sec", 6},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		joins, selects := 0, 0
		for _, err := range errors {
			if !strings.Contains(err.Message, "remains active across") || err.PreviousLine == 0 {
				t.Fatal(err)
			}
			if strings.Contains(err.Message, "across join") {
				joins++
			}
			if strings.Contains(err.Message, "across select") {
				selects++
			}
		}
		if tc.errors > 0 && (joins != 5 || selects != 1) {
			t.Fatal(joins, selects)
		}
	}
}
