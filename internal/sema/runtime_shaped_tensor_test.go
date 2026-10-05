package sema

import (
	"strings"
	"testing"
)

// Runtime-shaped owning tensors retain their own nominal tensor identity,
// expose their compile-time rank, and do not acquire the runtime-only
// Shape/Len facts before those separate semantics are implemented.
//
// Rules:
//   - rules/collections/shaped-types.md — §§3.4–3.5 "Runtime-shaped owning tensor" and "tensor_view"
//   - rules/collections/shaped-types.md — §5.1 "Type-level access"
func TestRuntimeShapedOwningTensorIdentityAndRank(t *testing.T) {
	input := `
module main

fn Inspect(value: tensor[float32, Shape[3]], view: tensor_view[float32, 3]) uint {
	return value.Rank
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	fn := analyzer.functions["Inspect"][0]
	owned := fn.Parameters[0].Type
	view := fn.Parameters[1].Type
	if got := typeDisplayName(owned); got != "tensor[float32, Shape[3]]" {
		t.Fatalf("runtime-shaped owner = %q", got)
	}
	if owned.Name != "tensor" || view.Name != "tensor_view" || sameConcreteType(owned, view) {
		t.Fatalf("owner/view identities were conflated: owner=%+v view=%+v", owned, view)
	}
	if owned.StaticElementCount != nil {
		t.Fatalf("runtime-shaped tensor has static element count %s", owned.StaticElementCount)
	}
	rank, ok := compilerKnownMember(owned, "Rank", true)
	if !ok || rank.Result.Name != "uint" || !strings.Contains(rank.Documentation, "3") {
		t.Fatalf("runtime-shaped tensor Rank = %+v, %v", rank, ok)
	}
	if _, ok := compilerKnownMember(owned, "Shape", true); ok {
		t.Fatal("runtime-shaped tensor type must not synthesize a complete runtime Shape value")
	}
	if _, ok := compilerKnownMember(owned, "Len", true); ok {
		t.Fatal("runtime-shaped tensor type must not synthesize a runtime Len")
	}
}

// Mixing the runtime Shape form with static extents is not a third tensor
// identity; it is rejected rather than silently reinterpreted.
//
// Rules:
//   - rules/collections/shaped-types.md — §§3.3–3.4 "tensor"
func TestRuntimeShapedOwningTensorRejectsMixedStaticExtents(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main
fn Invalid(value: tensor[float32, Shape[3], 4]) void { return }
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "requires exactly tensor[T, Shape[Rank]] with no static extents") {
		t.Fatalf("errors = %+v", errors)
	}
}
