package main

import (
	"os"
	"strings"
	"testing"
)

// TestChildPanicDomainsLSP exposes child risk separately from the function's
// synchronous panic proof and invalidates the displayed facts on replacement.
// Rules: rules/errors/panic.md §§6–9,21,28(7); lsp.md — Hover.
func TestChildPanicDomainsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/child_panic_domains_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///child-panic-domains/main.sec"
	for _, name := range []string{"SpawnTask", "SpawnThread", "SpawnLambda", "SpawnThreadLambda", "SpawnValue", "SpawnGeneric", "SpawnCaptured", "SpawnForeign", "Top"} {
		hover, ok := hoverForSource(uri, source, offsetPosition(source, strings.Index(source, "fn "+name+"(")+3))
		if !ok || !strings.Contains(hover.Contents.Value, "May panic: `no`") || !strings.Contains(hover.Contents.Value, "Child panic domain spawn-") || !strings.Contains(hover.Contents.Value, "containment follows target policy") {
			t.Fatal(name, hover, ok)
		}
	}
	replaced := strings.ReplaceAll(source, `panic "child panic"`, `return 1`)
	hover, ok := hoverForSource(uri, replaced, offsetPosition(replaced, strings.Index(replaced, "fn SpawnTask(")+3))
	if !ok || strings.Contains(hover.Contents.Value, "Child panic domain") {
		t.Fatal("stale child panic", hover, ok)
	}
	invalid, err := os.ReadFile("../../testdata/sema/child_panic_domains_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			errors++
			if d.Code != "S1092" {
				t.Fatal(d)
			}
		}
	}
	if errors != 2 {
		t.Fatal(errors)
	}
}
