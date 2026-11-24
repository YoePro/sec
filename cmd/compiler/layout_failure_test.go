package main

import (
	"strings"
	"testing"
)

// CLI analysis exposes an unimplemented descriptor layout without substituting
// scalar layout facts; its diagnostic remains an implementation limitation.
// Rules: MD-141 correction §10(1); rules/memory/layout.md — §§5,42.
func TestLayoutFailureContextCLI(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/layout_failure_descriptor.sec", "--target", "baremetal-cortex-m3")
	text := stdout + stderr
	if code != 0 {
		t.Fatalf("exit %d: %s", code, text)
	}
	for _, want := range []string{"layout of string", "baremetal/cortex-m3", "triple thumbv7m-none-eabi", "profile freestanding", "Arena.New needs a complete layout", "descriptor", "implement the required P24"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q: %s", want, text)
		}
	}
}
