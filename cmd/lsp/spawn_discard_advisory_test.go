package main

import (
	"os"
	"strings"
	"testing"
)

// LSP transports the shared occurrence and help, without a second spawn rule
// or source rewrite. Rules: rules/tooling/diagnostics.md §25(8); spawn.md §3A(8).
func TestSpawnDiscardAdvisoryLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks/advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	items := analyze("file:///spawn-discard/main.sec", string(data))
	count := 0
	for _, d := range items {
		if d.Code != "A3001" || d.Severity != 3 || d.Range.End.Character <= d.Range.Start.Character {
			t.Fatal(d)
		}
		if strings.Contains(d.Message, "successful handle") {
			count++
			if !strings.Contains(d.Message, "A final call is not an implicit return") || !strings.Contains(d.Message, "discard` before the call") {
				t.Fatal(d)
			}
		}
	}
	if count != 5 || len(items) != 7 {
		t.Fatal(items)
	}
}
