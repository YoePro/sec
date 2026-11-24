package main

import (
	"os"
	"strings"
	"testing"
)

// TestSpawnGrammarLSP shares canonical syntax formatting and distinguishes
// parser recovery from unsupported spawned-block semantics.
// Rules: rules/concurrency/spawn.md — §2; rules/foundations/grammar.md — §89;
// rules/tooling/formatter.md — §28; rules/tooling/lsp.md — diagnostics.
func TestSpawnGrammarLSP(t *testing.T) {
	assertFormattingGoldenLSP(t, "spawn_forms")
	for _, tc := range []struct{ file, message string }{
		{"../../testdata/parser/spawn_missing_operand_invalid.sec", "spawn requires an expression or block"},
		{"../../testdata/sema/spawn_blocks_invalid.sec", "compiler capability unavailable: spawned block body"},
	} {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range analyze("file:///spawn-grammar/main.sec", string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			lifecycle := strings.HasSuffix(tc.file, "spawn_blocks_invalid.sec") && strings.Contains(d.Message, "unresolved at scope exit")
			if (!strings.Contains(d.Message, tc.message) && !lifecycle) || strings.Contains(d.Message, "deprecated") {
				t.Fatal(d)
			}
		}
		if count != 4 {
			t.Fatal(tc.file, count)
		}
	}
}

// The LSP consumes shared Sema typing of void child blocks without guessing
// that the final call's result is the child return type.
// Rules: rules/concurrency/spawn.md §§3A(3),25(5); lsp.md — One semantic source of truth.
func TestVoidSpawnBlocksLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks/void.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range analyze("file:///spawn-blocks/main.sec", string(data)) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
}
