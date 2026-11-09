package sema

import (
	"os"
	"strings"
	"testing"
)

// TestThreadLegacyErrorsRemoved keeps legacy implementation-only identities
// out of the portable type/error catalog without reserving ordinary project
// names. Attachment errors retain their separately owned canonical identity.
// Rules: rules/concurrency/threads.md — §3(1–5);
// rules/concurrency/thread_local.md — §§29,37(1–4).
func TestThreadLegacyErrorsRemoved(t *testing.T) {
	names := []string{"ThreadSchedulingError", "ThreadTerminationError", "ThreadContextError"}
	for _, name := range names {
		if typ, exists := builtinTypes()[name]; exists {
			t.Fatal("legacy builtin", name, typ)
		}
	}
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"thread_legacy_errors_valid.sec", 0}, {"thread_legacy_errors_invalid.sec", 3}, {"thread_legacy_errors_local.sec", 0},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		if tc.errors > 0 {
			for i, name := range names {
				if !strings.Contains(errors[i].Message, "unknown type "+name) {
					t.Fatal(errors)
				}
			}
		}
		if strings.HasSuffix(tc.file, "_local.sec") {
			for _, name := range names {
				typ := a.Types()[name]
				if typ.Intrinsic || typ.Module != "main" || !typ.ErrorAssignable {
					t.Fatal(name, typ)
				}
			}
		} else {
			for _, name := range names {
				if _, exists := a.Types()[name]; exists {
					t.Fatal("legacy identity survived", name)
				}
			}
		}
	}
}
