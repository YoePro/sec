package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLSPGenericRecursiveStorageBoundaries consumes the compiler's distinction
// between finite storage descriptors and recursive by-value layout.
// Rules: rules/declarations/unions.md — §18; rules/memory/layout.md — §25;
// rules/tooling/lsp.md — Shared diagnostic model.
func TestLSPGenericRecursiveStorageBoundaries(t *testing.T) {
	for _, suffix := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/generics/recursive_boundaries_" + suffix + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "boundaries.sec"))
		values := analyze(uri, string(data))
		if suffix == "valid" {
			if len(values) != 0 {
				t.Fatal(values)
			}
			continue
		}
		if len(values) != 4 {
			t.Fatal(values)
		}
		for _, value := range values {
			if value.Severity != 1 || !strings.Contains(value.Message, "infinite size") {
				t.Fatal(value)
			}
		}
	}
}
