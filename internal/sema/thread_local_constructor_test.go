package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestThreadLocalConstructor checks concrete initial-value eligibility and argument validation.
// Rules: rules/concurrency/thread_local.md — §7(1–6).
func TestThreadLocalConstructor(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_local_constructor_valid.sec", 0}, {"thread_local_constructor_invalid.sec", 6},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		for i, err := range errors {
			if i < 2 && (err.ID != diagnostics.ThreadLocalInitialNotCopyable || !strings.Contains(err.Help, "factory")) {
				t.Fatal(err)
			}
		}
	}
}
