package main

import (
	"os"
	"strings"
	"testing"
)

// TestMutexJoinBoundaryLSP projects live-guard wait diagnostics and related guard declarations.
// Rules: rules/concurrency/mutex.md — §41(2–3); rules/tooling/lsp.md — diagnostics.
func TestMutexJoinBoundaryLSP(t *testing.T) {
	uri := "file:///mutex-join/main.sec"
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
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if !strings.Contains(d.Message, "remains active across") || len(d.RelatedInformation) != 1 {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
