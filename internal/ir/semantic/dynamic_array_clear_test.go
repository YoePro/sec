package semantic

import (
	"strings"
	"testing"
)

// `values.Clear()` on a dynamic array with a trivially destructible element
// lowers to one dynamic-array.clear on the writable owner Place: no
// allocation, growth, epoch change or failure path. A non-trivially
// destructible element stays a named boundary.
// Rules: rules/collections/collections.md — § 6.8.
func TestDynamicArrayClearSemanticIR(t *testing.T) {
	module, err := hostedModule(t, `module main

fn Fill() Result[uint, CollectionError] {
    let mut values: int[]
    try values.Append(4)
    values.Clear()
    return Ok(values.Len)
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
	if strings.Count(text, "dynamic-array.clear %") != 1 {
		t.Fatalf("want one dynamic-array.clear:\n%s", text)
	}
	after := text[strings.Index(text, "dynamic-array.clear"):]
	for _, unwanted := range []string{"dynamic-array.grow-backing", "storage.advance-epoch", "storage.end-domain"} {
		if strings.Contains(after[:strings.Index(after, "dynamic-array.len")], unwanted) {
			t.Fatalf("Clear must not emit %q:\n%s", unwanted, text)
		}
	}
	fn := loopFunction(t, module, "Fill")
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpDynamicArrayClear {
				op.Operands = nil
				if err := verifyDynamicArrayStorageOperation(module.Types, op, nil); err == nil {
					t.Fatal("verifier accepted Clear without an owner Place")
				}
			}
		}
	}

	if strings.Contains(text, "element-plan=") {
		t.Fatalf("a trivially destructible element carries no plan:\n%s", text)
	}

	// An element that needs destruction names its element plan, the same
	// plan the array's destroy-elements-reverse step uses.
	nested, err := hostedModule(t, "module main\n\nfn F() void {\n    let mut values: int[][]\n    values.Clear()\n}\n\nfn main() int {\n    return 0\n}\n", 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(nested); err != nil {
		t.Fatal(err)
	}
	nestedText := Format(nested)
	if !strings.Contains(nestedText, `dynamic-array.clear %`) || !strings.Contains(nestedText, `element-plan="destroy:array:dynamic::int:int"`) {
		t.Fatalf("Clear lacks its element plan:\n%s", nestedText)
	}
	for _, block := range loopFunction(t, nested, "F").Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpDynamicArrayClear {
				continue
			}
			values := map[ValueID]Value{}
			for _, b := range loopFunction(t, nested, "F").Blocks {
				for _, o := range b.Operations {
					for _, r := range o.Results {
						values[r.ID] = r
					}
				}
			}
			stripped := op
			stripped.Ownership = nil
			if err := verifyClearElementPlan(nested, stripped, values); err == nil {
				t.Fatal("verifier accepted Clear of non-trivial elements without their plan")
			}
			wrong := op
			wrong.Ownership = &OwnershipFacts{DestructionPlan: "destroy::int:int"}
			if err := verifyClearElementPlan(nested, wrong, values); err == nil {
				t.Fatal("verifier accepted a plan of another type")
			}
		}
	}
}
