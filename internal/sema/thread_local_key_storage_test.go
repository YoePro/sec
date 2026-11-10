package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestThreadLocalKeyStorage checks static keys and rejects automatic ownership.
// Rules: rules/concurrency/thread_local.md — §20(1–4).
func TestThreadLocalKeyStorage(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_local_key_storage_valid.sec", 0}, {"thread_local_key_storage_invalid.sec", 4},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		for _, err := range errors {
			if err.ID != diagnostics.ThreadLocalAutomaticKey || !strings.Contains(err.Help, "static let") {
				t.Fatal(err)
			}
		}
	}
}
