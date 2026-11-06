package sema

import (
	"sort"
	"testing"
)

func symbolTableNames(t *symbolTable) []string {
	names := []string{}
	for name := range t.all() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestSymbolTableLayersBehaveLikeScopeCopies checks that a child layer reads
// through to its parent while its writes and removals stay in the child, as
// the former per-scope map copies did.
func TestSymbolTableLayersBehaveLikeScopeCopies(t *testing.T) {
	root := symbolTableOf(map[string]Symbol{"global": {Mutable: false}, "shadowed": {Mutable: false}})
	scope := root.child()
	scope.set("local", Symbol{Mutable: true})
	scope.set("shadowed", Symbol{Mutable: true})
	scope.del("global")

	if scope.has("global") || !root.has("global") {
		t.Fatalf("removal in a child must hide the parent's symbol only in the child")
	}
	if !scope.get("shadowed").Mutable || root.get("shadowed").Mutable {
		t.Fatalf("a child write must shadow, not change, the parent's symbol")
	}
	if root.has("local") {
		t.Fatalf("a child definition leaked into the parent")
	}
	if got := symbolTableNames(scope); len(got) != 2 || got[0] != "local" || got[1] != "shadowed" {
		t.Fatalf("child iteration = %v, want [local shadowed]", got)
	}
	if scope.len() != 2 || root.len() != 2 {
		t.Fatalf("len: child %d, root %d", scope.len(), root.len())
	}
	scope.set("global", Symbol{Mutable: true})
	if !scope.has("global") || !scope.get("global").Mutable {
		t.Fatalf("redefining a hidden name must make it visible again")
	}

	// A loop-header snapshot forks the current layer: both continuations get
	// their own child and neither sees the other's writes.
	snapshot, continuing := scope.child(), scope.child()
	continuing.set("afterSnapshot", Symbol{})
	if snapshot.has("afterSnapshot") {
		t.Fatalf("a write after the fork reached the snapshot")
	}

	var nilTable *symbolTable
	if nilTable.len() != 0 || nilTable.has("x") || nilTable.child() == nil {
		t.Fatalf("a nil table must behave as an empty one")
	}
}

// TestSymbolTableDepthIsBounded checks that a deep chain is flattened without
// changing what it contains.
func TestSymbolTableDepthIsBounded(t *testing.T) {
	table := newSymbolTable()
	table.set("base", Symbol{})
	for range 3 * maxSymbolTableDepth {
		table = table.child()
		table.set("top", Symbol{Mutable: table.depth%2 == 0})
	}
	if table.depth > maxSymbolTableDepth {
		t.Fatalf("depth %d exceeds %d", table.depth, maxSymbolTableDepth)
	}
	if got := symbolTableNames(table); len(got) != 2 || got[0] != "base" || got[1] != "top" {
		t.Fatalf("flattened chain = %v", got)
	}
}
