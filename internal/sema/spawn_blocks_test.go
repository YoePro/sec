package sema

import (
	"os"
	"strings"
	"testing"
)

// TestSpawnBlockCapabilityBoundary keeps valid block grammar distinct from
// the unimplemented child-body/capture capability for every execution kind.
// Rules: rules/concurrency/spawn.md — §2(2–3);
// rules/foundations/grammar.md — §89.
func TestSpawnBlockCapabilityBoundary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 4 {
		t.Fatal(errors)
	}
	for i, kind := range []string{"task", "task", "thread", "process"} {
		if !strings.Contains(errors[i].Message, "compiler capability unavailable: spawned block body for "+kind) || !strings.Contains(errors[i].Help, "valid spawn syntax") {
			t.Fatal(errors[i])
		}
	}
}
