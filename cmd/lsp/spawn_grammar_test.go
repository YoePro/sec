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
			if !strings.Contains(d.Message, tc.message) || strings.Contains(d.Message, "deprecated") {
				t.Fatal(d)
			}
		}
		if count != 4 {
			t.Fatal(tc.file, count)
		}
	}
}
