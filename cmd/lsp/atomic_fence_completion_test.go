package main

import (
	"strings"
	"testing"
)

// `atomic.` completes the core namespace function Fence, and MemoryOrder
// completion inside atomic.Fence(...) offers only the valid fence orders; a
// local value named atomic keeps ordinary member completion.
//
// Rules:
//   - rules/concurrency/atomics.md — §§ 3(5), 26
//   - rules/tooling/lsp.md — "Completion"
func TestAtomicFenceCompletion(t *testing.T) {
	labels := func(source string) []string {
		t.Helper()
		items := completeSource("", source, strings.LastIndex(source, ".")+1)
		result := []string{}
		for _, item := range items {
			result = append(result, item.Label)
		}
		return result
	}
	if got := labels("module main\n\nfn Publish() void {\n    atomic.\n}\n"); strings.Join(got, ",") != "Fence" {
		t.Fatalf("atomic. completion = %v", got)
	}
	orders := strings.Join(labels("module main\n\nfn Publish() void {\n    atomic.Fence(MemoryOrder.)\n}\n"), ",")
	for _, want := range []string{"Acquire", "Release", "AcqRel", "SeqCst"} {
		if !strings.Contains(orders, want) {
			t.Errorf("fence orders lack %s: %s", want, orders)
		}
	}
	if strings.Contains(orders, "Relaxed") {
		t.Errorf("Relaxed offered for atomic.Fence: %s", orders)
	}
	if ordinary := strings.Join(labels("module main\n\nfn Load(counter: ref Atomic[uint64]) uint64 {\n    return counter.Load(MemoryOrder.)\n}\n"), ","); !strings.Contains(ordinary, "Relaxed") {
		t.Errorf("Relaxed must stay available outside fences: %s", ordinary)
	}
	shadowed := strings.Join(labels("module main\n\ntype Barrier struct {\n    count: int,\n}\n\nfn Use(atomic: Barrier) int {\n    return atomic.\n}\n"), ",")
	if strings.Contains(shadowed, "Fence") || !strings.Contains(shadowed, "count") {
		t.Errorf("a local atomic value must complete its own members: %s", shadowed)
	}
}
