package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadLocalConstructorLSP projects canonical constructor diagnostics.
// Rules: rules/concurrency/thread_local.md — §7(1–6); rules/tooling/lsp.md — diagnostics.
func TestThreadLocalConstructorLSP(t *testing.T) {
	uri := "file:///tls-key-storage/main.sec"
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
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if strings.Contains(d.Message, "implicitly copyable") && d.Code != "S1209" {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
