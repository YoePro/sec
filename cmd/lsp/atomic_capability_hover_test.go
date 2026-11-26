package main

import (
	"strings"
	"testing"
)

// TestAtomicCapabilityHover: hover on an atomic operation lists its target
// capability per execution context and presents a missing plan fact as
// unproven, never as executable validity or a proven unsupported capability
// (MD-095 T51, T53).
// Rules: rules/concurrency/atomics.md — §§ 37A(12), 48(4).
func TestAtomicCapabilityHover(t *testing.T) {
	source := "module main\n\nfn Publish() void {\n    atomic.Fence(MemoryOrder.Release)\n}\n\nfn main() int {\n    Publish()\n    return 0\n}\n"
	uri := "file:///tmp/atomic_capability_hover.sec"
	offset := strings.Index(source, "Fence(") + 1
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok {
		t.Fatal("no hover")
	}
	text := hover.Contents.Value
	if !strings.Contains(text, "**Target capability**") || !strings.Contains(text, "- Ordinary: unproven, ") || strings.Contains(text, "not supported") || strings.Contains(text, "supported by") {
		t.Fatalf("capability hover:\n%s", text)
	}
}
