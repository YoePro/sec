package main

import (
	"os"
	"strings"
	"testing"
)

// TestCTETransientStorageLSP exposes the compiler's resource category and the
// source of failure; hover never attempts a separate evaluation or fallback.
// Rules: rules/compiler/compile_time_evaluation.md — §§48–49,59,73;
// rules/tooling/lsp.md — Diagnostics, Hover.
func TestCTETransientStorageLSP(t *testing.T) {
	uri := "file:///cte-transient-storage/main.sec"
	file := "../../testdata/sema/cte_transient_storage_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	errors := 0
	for _, d := range analyze(uri, source) {
		if d.Severity != 1 {
			continue
		}
		errors++
		if d.Code != "S1225" && d.Code != "S1224" {
			t.Fatal(d)
		}
	}
	if errors != 3 {
		t.Fatal(errors)
	}
	for _, item := range []struct{ marker, want string }{
		{"Grow(40)", "ResourceLimit"}, {"Churn()\nlet Independent", "TransientMemory"},
		{"ExplicitResource()\n", "ForbiddenEffect"},
	} {
		offset := strings.LastIndex(source, item.marker)
		if offset < 0 {
			t.Fatal("missing marker", item.marker)
		}
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, item.want) {
			t.Fatal(item.marker, hover, ok)
		}
	}
}
