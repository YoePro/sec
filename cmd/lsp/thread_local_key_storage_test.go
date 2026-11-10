package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadLocalKeyStorageLSP projects canonical static-key diagnostics.
// Rules: rules/concurrency/thread_local.md — §20(1–4); rules/tooling/lsp.md — diagnostics.
func TestThreadLocalKeyStorageLSP(t *testing.T) {
	uri := "file:///tls-key-storage/main.sec"
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
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1208" || !strings.Contains(d.Message, "cannot own a ThreadLocal key") {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
