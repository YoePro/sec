package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Active collection iteration forbids structural mutation and replacement of
// the iterated collection or any storage enclosing it, while element mutation,
// mutation inside one element, sibling storage, and code after the loop stay
// valid.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §7, §8 "Structural stability during iteration", §16, §41
//   - rules/compiler/compiler_analysis.md — § 18(2) structural mutation dependencies
func TestStructuralMutationDuringIterationIsRejected(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "append during plain iteration", body: "for item in items {\n        try items.Append(item)\n    }", want: "cannot Append items while the enclosing for loop iterates items"},
		{name: "append during ref mut iteration", body: "for ref mut item in items {\n        try items.Append(1)\n    }", want: "cannot Append items"},
		{name: "clear during ref iteration", body: "for ref item in items {\n        items.Clear()\n    }", want: "cannot Clear items"},
		{name: "remove at during indexed iteration", body: "for i, item in items {\n        discard items.RemoveAt(i)\n    }", want: "cannot RemoveAt items"},
		{name: "replace whole collection", body: "for item in items {\n        items = [1]\n    }", want: "cannot assign items while the enclosing for loop iterates items"},
		{name: "map remove", body: "for k, v in lookup {\n        discard lookup.Remove(k)\n    }", want: "cannot Remove lookup"},
		{name: "set add", body: "for value in seen {\n        try seen.Add(value + 1)\n    }", want: "cannot Add seen"},
		{name: "field collection", body: "for item in store.items {\n        store.items.Clear()\n    }", want: "cannot Clear store.items while the enclosing for loop iterates store.items"},
		{name: "enclosing storage replaced", body: "for item in store.items {\n        store = Store {}\n    }", want: "cannot assign store while the enclosing for loop iterates store.items"},
		{name: "outer loop source from inner loop", body: "for item in items {\n        for ref row in grid {\n            items.Clear()\n        }\n    }", want: "cannot Clear items"},
		{name: "collection behind mutable reference parameter", body: "for value in holder.items {\n        try holder.items.Append(value)\n    }", want: "cannot Append holder.items"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeStructuralMutationFixture(t, test.body)
			if len(errors) != 1 || errors[0].ID != diagnostics.StructuralMutationDuringIteration || errors[0].Help == "" || errors[0].PreviousLine == 0 || !strings.Contains(errors[0].Message, test.want) {
				t.Fatalf("errors = %+v, want one S1075 containing %q with help and the loop location", errors, test.want)
			}
		})
	}
}

func TestNonStructuralMutationDuringIterationRemainsValid(t *testing.T) {
	for _, body := range []string{
		"for item in items {\n        items[0] = item\n    }",
		"for ref mut row in grid {\n        try row.Append(1)\n    }",
		"for ref row in grid {\n        try grid[0].Append(1)\n    }",
		"for item in store.items {\n        store.count += 1\n    }",
		"for item in items {\n        try other.Append(item)\n    }",
		"for item in items {\n        discard item\n    }\n    try items.Append(1)",
		"for i in uint(0)..<items.Len {\n        try items.Append(1)\n    }",
	} {
		if errors := analyzeStructuralMutationFixture(t, body); len(errors) != 0 {
			t.Fatalf("body %q errors = %v, want none", body, errors)
		}
	}
}

func analyzeStructuralMutationFixture(t *testing.T, body string) []Error {
	t.Helper()
	return analyzeSourceRaw(t, `module main
type Store struct {
    items: int[],
    count: int
}
fn Use(lookup: ref mut map[int, int], seen: ref mut set[int], holder: ref mut Store) Result[void, CollectionError] {
    let mut items: int[] := [1, 2]
    let mut other: int[] := [0]
    let mut grid: int[][] := [[1]]
    let mut store := Store {}
    `+body+`
    return Ok()
}
`)
}
