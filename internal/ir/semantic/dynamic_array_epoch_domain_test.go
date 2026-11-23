package semantic

import (
	"strings"
	"testing"
)

// package18 §§ 47, 77, 100-101: a direct element borrow and a slice of an
// owning T[] depend on the owner's backing incarnation. They stay proven
// (Sema rejects growth while they live, S1162) and name the owner's backing
// domain; the verifier rejects an element borrow naming another domain.
func TestDynamicArrayBorrowsNameBackingDomain(t *testing.T) {
	module, err := hostedModule(t, `module main

fn Count(view: ref int[]) uint {
    return view.Len
}

fn Fill() Result[int, CollectionError] {
    let mut values: int[]
    try values.Append(4)
    let element := ref values[0]
    let second := *element
    let total := Count(ref values[..])
    discard total
    return Ok(second)
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
	for _, want := range []string{
		"backing=#1}", "ref.borrow-shared %21 {borrow=#3 lifetime=#3 validity=proven proof=lexical-borrow domain=#1 ",
		"slice.borrow-shared", "validity=proven proof=lexical-borrow domain=#1 space=ordinary",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				if op := &block.Operations[index]; op.Kind == OpReferenceBorrowShared && op.Reference != nil && op.Reference.InvalidationDomain == 1 {
					op.Reference.InvalidationDomain = 7
				}
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "not the element Place's backing domain #1") {
		t.Fatalf("mismatched element borrow domain accepted: %v", err)
	}
}
