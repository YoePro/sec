package semantic

import (
	"strings"
	"testing"
)

// `values.RemoveAt(index)` on a trivially copyable element splits on the
// total index-in-bounds predicate: the true edge extracts with
// dynamic-array.remove-at and builds Some, the false edge builds None without
// mutation. No panic, allocation or epoch change exists. Other elements are
// extracted as owned values.
// Rules: rules/collections/collections.md — § 6.9.
func TestDynamicArrayRemoveAtSemanticIR(t *testing.T) {
	module, err := hostedModule(t, `module main

fn Take(at: uint) Result[int, CollectionError] {
    let mut values: int[]
    try values.Append(4)
    let removed := values.RemoveAt(at)
    values.RemoveAt(0)
    if removed is Some(value) {
        return Ok(value)
    }
    return Ok(0)
}

fn main() int {
    return 0
}
`, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if strings.Count(text, "dynamic-array.remove-at %") != 2 || strings.Count(text, "dynamic-array.index-in-bounds") != 2 {
		t.Fatalf("want two guarded remove-at operations:\n%s", text)
	}
	for _, unwanted := range []string{"fail.bounds", "panic", "storage.advance-epoch"} {
		if strings.Contains(text[strings.Index(text, "dynamic-array.remove-at"):], unwanted) {
			t.Fatalf("RemoveAt must not lower %q:\n%s", unwanted, text)
		}
	}
	fn := loopFunction(t, module, "Take")
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpDynamicArrayRemoveAt {
				values := map[ValueID]Value{}
				for _, b := range fn.Blocks {
					for _, p := range b.Parameters {
						values[p.ID] = p
					}
					for _, o := range b.Operations {
						for _, r := range o.Results {
							values[r.ID] = r
						}
					}
				}
				unguarded := op
				unguarded.ArrayGuard = 0
				if err := verifyDynamicArrayStorageOperation(module.Types, unguarded, values); err == nil {
					t.Fatal("verifier accepted an unguarded remove-at")
				}
			}
		}
	}
	// An owned element leaves the array as an owned value the Option
	// constructs directly; an unused result is implicitly discarded with the
	// Option's destruction plan.
	nested, err := hostedModule(t, "module main\n\nfn F() void {\n    let mut values: int[][]\n    let removed := values.RemoveAt(0)\n    discard removed\n    values.RemoveAt(1)\n}\n\nfn main() int {\n    return 0\n}\n", 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(nested); err != nil {
		t.Fatal(err)
	}
	nestedText := Format(nested)
	for _, want := range []string{
		"dynamic-array.remove-at %", "[owned]", "[construct-direct]",
		`own.discard %5 plan="destroy::union:Option[array:dynamic::int:int]" origin=explicit`,
		`plan="destroy::union:Option[array:dynamic::int:int]" origin=implicit-call-result`,
	} {
		if !strings.Contains(nestedText, want) {
			t.Fatalf("missing %q in:\n%s", want, nestedText)
		}
	}
}
