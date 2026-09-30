package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Struct layout validation uses declaration dependencies, so direct, forward,
// mutual, and fixed-array-mediated cycles are rejected while canonical
// indirection boundaries remain legal.
//
// Rules:
//   - rules/declarations/struct.md — §20 "Semantic analysis requirements"
//   - rules/memory/layout.md — §25 "Recursive layout"
//   - rules/memory/layout.md — §44(3) "Recursive/generic tests"
func TestStructLayoutCyclesAreRejectedWithPath(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantPath string
	}{
		{
			name: "direct",
			source: `type Node struct {
    next: Node,
}`,
			wantPath: "Node.next -> Node",
		},
		{
			name: "mutual forward declarations",
			source: `type Left struct {
    right: Right,
}
type Right struct {
    left: Left,
}`,
			wantPath: "Left.right -> Right.left -> Left",
		},
		{
			name: "fixed array mediated",
			source: `type Node struct {
    children: Node[2],
}`,
			wantPath: "Node.children -> Node",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errors := analyzeSourceWithAnalyzer(t, test.source)
			var found *Error
			for index := range errors {
				if errors[index].ID == diagnostics.RecursiveStructLayout {
					found = &errors[index]
					break
				}
			}
			if found == nil {
				t.Fatalf("errors = %v, want %s", errors, diagnostics.RecursiveStructLayout)
			}
			if !strings.Contains(found.Message, test.wantPath) || found.Help == "" {
				t.Fatalf("diagnostic = %+v, want path %q and help", *found, test.wantPath)
			}
		})
	}
}

func TestStructLayoutIndirectionBreaksRecursion(t *testing.T) {
	_, errors := analyzeSourceWithAnalyzer(t, `type RefNode struct {
    next: ref RefNode,
}
type DynamicNode struct {
    children: DynamicNode[],
}`)
	if len(errors) != 0 {
		t.Fatalf("indirected recursion errors = %v", errors)
	}
}
