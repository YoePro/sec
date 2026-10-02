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
		{
			name: "named wrapper mediated",
			source: `type Node struct {
    wrapped: Wrapper,
}
type Wrapper Node`,
			wantPath: "Node.wrapped -> Wrapper -> Node",
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

// The by-value layout graph also covers tagged unions, compiler-known tagged
// Option/Result payloads, and cycles that only generic instantiation
// introduces, reporting the variant and instantiation path.
//
// Rules:
//   - rules/memory/layout.md — §15(7), §16(1), §25(1)-(5), §42(4), §44(3)
//   - rules/declarations/unions.md — §18 "Recursive unions"; §26 "Required diagnostics"
func TestByValueLayoutGraphCoversUnionsTaggedPayloadsAndInstantiation(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantID   string
		wantPath string
	}{
		{
			name: "option payload",
			source: `type Node struct {
    next: Option[Node],
}`,
			wantID: diagnostics.RecursiveStructLayout, wantPath: "Node.next (via Option[Node]) -> Node",
		},
		{
			name: "result error payload",
			source: `type Node struct {
    next: Result[int, Node],
}`,
			wantID: diagnostics.RecursiveStructLayout, wantPath: "Node.next (via Result[int, Node]) -> Node",
		},
		{
			name: "instantiation introduces the cycle",
			source: `type Holder[T] struct {
    value: T,
}
type Node struct {
    held: Holder[Node],
}`,
			wantID: diagnostics.RecursiveStructLayout, wantPath: "Node.held (via Holder[Node]) -> Node",
		},
		{
			name: "parameter stored through another generic",
			source: `type Holder[T] struct {
    value: T,
}
type Outer[T] struct {
    inner: Holder[T],
}
type Node struct {
    outer: Outer[Node],
}`,
			wantID: diagnostics.RecursiveStructLayout, wantPath: "Node.outer (via Outer[Node]) -> Node",
		},
		{
			name: "mutual generic declarations",
			source: `type Left[T] struct {
    right: Right[T],
}
type Right[T] struct {
    left: Left[T],
}`,
			wantID: diagnostics.RecursiveStructLayout, wantPath: "Left.right -> Right.left -> Left",
		},
		{
			name: "mutual unions through payloads",
			source: `type Tree union {
    Leaf(int),
    Branch(Forest),
}
type Forest union {
    One(Tree),
    Empty,
}`,
			wantID: diagnostics.RecursiveUnionLayout, wantPath: "Tree.Branch -> Forest.One -> Tree",
		},
		{
			name: "union payload field through struct",
			source: `type Figure union {
    Group {
        inner: Wrapper,
    },
    Dot,
}
type Wrapper struct {
    figure: Figure,
}`,
			wantID: diagnostics.RecursiveStructLayout, wantPath: "Figure.Group.inner -> Wrapper.figure -> Figure",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errors := analyzeSourceWithAnalyzer(t, test.source)
			var found []Error
			for _, err := range errors {
				if err.ID == test.wantID {
					found = append(found, err)
				}
			}
			if len(found) != 1 {
				t.Fatalf("errors = %v, want exactly one %s", errors, test.wantID)
			}
			if !strings.Contains(found[0].Message, test.wantPath) || found[0].Help == "" {
				t.Fatalf("diagnostic = %+v, want path %q and help", found[0], test.wantPath)
			}
		})
	}
}

// Boundaries inside tagged and generic carriers stay legal, generic
// parameters stored only behind indirection never create edges, and a direct
// self-recursive union keeps exactly one diagnostic.
func TestByValueLayoutGraphRespectsBoundariesAndSingleRootCause(t *testing.T) {
	_, errors := analyzeSourceWithAnalyzer(t, `type Indirect[T] struct {
    items: T[],
}
type Node struct {
    next: Option[Node[]],
    indirect: Indirect[Node],
    parent: Option[ref Node],
}
type List[T] union {
    Cell {
        value: T,
        next: List[T][],
    },
    End,
}`)
	if len(errors) != 0 {
		t.Fatalf("indirected recursion errors = %v", errors)
	}

	_, errors = analyzeSourceWithAnalyzer(t, `type Loop union {
    Again(Loop),
    Stop,
}`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "infinite size") {
		t.Fatalf("direct union recursion errors = %v, want exactly one", errors)
	}
}
