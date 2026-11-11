package main

import (
	"strings"
	"testing"
)

// Analysis renders the same canonical pointer facts as editor hover.
// Rules: rules/memory/raw_pointers.md §35(4–6).
func TestRawPointerFactAnalysis(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/tooling/raw_pointer_facts.sec")
	text := stdout + stderr
	if code != 0 {
		t.Fatalf("code %d: %s", code, text)
	}
	for _, want := range []string{"raw pointers:", "Nullability: proven null", "Nullability: not established", "Known origin: `session`", "Origin no longer holds a value", "Address space, range, alignment and lifetime: not established"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
}
