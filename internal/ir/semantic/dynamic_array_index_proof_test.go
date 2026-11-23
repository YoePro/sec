package semantic

import (
	"strings"
	"testing"
)

// package18 § 41: a Sema-proven T[] index forms its element Place without
// index-in-bounds, guard or bounds failure; an index after Append keeps the
// guarded form. The verifier rejects a constant proof for a runtime length.
func TestDynamicArrayProvenIndexSemanticIR(t *testing.T) {
	module, err := hostedModule(t, `module main

fn Read(i: uint) Result[int, CollectionError] {
    let mut values: int[]
    try values.Append(3)
    if i < values.Len {
        let first := values[i]
        try values.Append(4)
        return Ok(first + values[i])
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
	if strings.Count(text, "dynamic-array.element-place") != 2 || strings.Count(text, "bounds=proven-safe proof=branch") != 1 ||
		strings.Count(text, "bounds=runtime-check proof=guarded") != 1 || strings.Count(text, "dynamic-array.index-in-bounds") != 1 {
		t.Fatalf("want one proven and one guarded element Place:\n%s", text)
	}
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				if op := &block.Operations[index]; op.Kind == OpDynamicArrayElementPlace && op.ArrayCheckKind == ArrayIndexProvenSafe {
					op.ArrayProofKind = ArrayIndexProofConstant
				}
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "cannot bound a runtime length") {
		t.Fatalf("constant proof of a runtime length accepted: %v", err)
	}
}
