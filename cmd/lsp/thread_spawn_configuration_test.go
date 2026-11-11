package main

import (
	"os"
	"strings"
	"testing"
)

// TestNamedThreadConfigurationLSP uses compiler ownership diagnostics and
// parser recovery without treating a named configuration as a child callable.
// Rules: rules/concurrency/threads.md — §§10(3),20; spawn.md — §4;
// rules/tooling/lsp.md — Diagnostics.
func TestNamedThreadConfigurationLSP(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"../../testdata/sema/thread_named_config_valid.sec", 0},
		{"../../testdata/sema/thread_named_config_invalid.sec", 7},
		{"../../testdata/parser/thread_config_missing_callable_invalid.sec", 1},
	} {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		count, marker := 0, false
		for _, d := range analyze("file:///thread-configuration/main.sec", string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if strings.Contains(d.Message, "explicit <- ownership transfer") {
				marker = true
				if d.Code == "" {
					t.Fatal("missing ownership diagnostic ID", d)
				}
			}
		}
		if count != tc.errors || strings.Contains(tc.file, "config_invalid") && !marker {
			t.Fatal(tc.file, count, marker)
		}
	}
}
