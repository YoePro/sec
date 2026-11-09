package main

import (
	"os"
	"strings"
	"testing"
)

// Hover consumes canonical operation effects, preserving ordinary/volatile
// distinctions without an editor-specific raw-pointer classifier.
// Rules: rules/memory/raw_pointers.md §31(6),§35(3);
// rules/tooling/lsp.md — "Hover".
func TestRawMemoryEffectHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/raw_memory_effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct{ member, effect string }{
		{"Write", "memory-write"}, {"Read", "memory-read"},
		{"VolatileWrite", "volatile-write"}, {"VolatileRead", "volatile-read"},
	} {
		offset := strings.Index(source, "."+tc.member+"(") + 1
		hover, found := hoverForSource("file:///tmp/raw-memory-effects/main.sec", source, offsetPosition(source, offset))
		if !found || !strings.Contains(hover.Contents.Value, "Effects: `"+tc.effect+"`") {
			t.Fatalf("%s hover lost canonical effect: %+v", tc.member, hover)
		}
	}
}
