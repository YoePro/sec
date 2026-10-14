package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLSPArenaMovedParameterDomain consumes canonical ArenaDomain and reset
// summaries for unsaved source, preserving both local and caller failures.
// Rules: rules/memory/arena.md — §§4.2,44,68; rules/tooling/lsp.md — Shared diagnostic model.
func TestLSPArenaMovedParameterDomain(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_domain_alias_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "arena.sec"))
	count := 0
	for _, value := range analyze(uri, string(data)) {
		if value.Severity != 1 {
			continue
		}
		count++
		if !strings.Contains(value.Message, "cannot use values after arena") || len(value.RelatedInformation) == 0 {
			t.Fatal(value)
		}
	}
	if count != 2 {
		t.Fatal(count)
	}
}
