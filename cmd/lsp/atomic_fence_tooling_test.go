package main

import (
	"strings"
	"testing"
)

// Hover over atomic.Fence shows the canonical signature, documentation, rule
// and the order the call establishes; go-to-definition reaches the canonical
// synthetic read-only definition of the compiler-known fence.
//
// Rules:
//   - rules/concurrency/atomics.md — §§ 3(5), 26, 40
//   - rules/compiler/compiler_known_members.md — "Synthetic definitions"
func TestAtomicFenceHoverAndNavigation(t *testing.T) {
	source := "module main\n\nfn Publish() void {\n    atomic.Fence(MemoryOrder.Release)\n}\n"
	uri := "file:///tmp/sec-atomic-fence/main.sec"
	offset := strings.Index(source, "Fence(") + 1
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok {
		t.Fatal("no hover on atomic.Fence")
	}
	for _, want := range []string{"fn Fence(order: MemoryOrder) void", "Relaxed is invalid", "This call establishes MemoryOrder.Release", "rules/concurrency/atomics.md"} {
		if !strings.Contains(hover.Contents.Value, want) {
			t.Errorf("hover lacks %q:\n%s", want, hover.Contents.Value)
		}
	}
	definitions := definitionsForSource(uri, source, offsetPosition(source, offset))
	if len(definitions) != 1 || !strings.HasPrefix(definitions[0].URI, compilerKnownScheme+":/CKF-ATOMIC-FENCE.sec") {
		t.Fatalf("definition: %+v", definitions)
	}
	text, ok := compilerKnownDefinitionText(definitions[0].URI)
	if !ok || !strings.Contains(text, "fn Fence(order: MemoryOrder) void") || !strings.Contains(text, "core atomic namespace") {
		t.Fatalf("synthetic definition:\n%s", text)
	}
}
