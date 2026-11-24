package sema

import (
	"os"
	"strings"
	"testing"
)

// TestSpawnBlockCapabilityBoundary keeps valid block grammar distinct from
// supported void children and the remaining process-body capability.
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
	for i := 0; i < 3; i++ {
		if !strings.Contains(errors[i].Message, "unresolved at scope exit") || strings.Contains(errors[i].Message, "capability unavailable") {
			t.Fatal(errors[i])
		}
	}
	if !strings.Contains(errors[3].Message, "compiler capability unavailable: spawned block body for process") {
		t.Fatal(errors[3])
	}
}
