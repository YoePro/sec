package semantic

import (
	"math/big"
	"strings"
	"testing"
)

// collections.md § 6.7, package18 §§ 35-37, 65-67: `values.Append(a..b)`
// grows once by the segment count (all or nothing) and appends the whole
// segment with one compact append-range-prepared; an empty segment
// establishes no backing and succeeds. The verifier rejects capacity
// ensured for a different count.
func TestDynamicArrayRangeAppendSemanticIR(t *testing.T) {
	source := `module main

fn Fill() Result[uint, CollectionError] {
    let mut values: int[]
    try values.Append(1..9)
    try values.Append(10..<10)
    let mut letters: rune[]
    try letters.Append('a'..'z')
    return Ok(values.Len + letters.Len)
}

fn main() int {
    return 0
}
`
	module, err := hostedModule(t, source, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"dynamic-array.grow-backing %2 count=9 {domain=#1 transition=establish-domain",
		"dynamic-array.has-capacity %2 count=9", "dynamic-array.grow-backing %2 count=9 {domain=#1 transition=advance-epoch",
		"dynamic-array.append-range-prepared %2, %3 count=9",
		"count=26", "dynamic-array.append-range-prepared",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "dynamic-array.append-range-prepared") != 2 || strings.Contains(text, "dynamic-array.append-prepared") {
		t.Fatalf("want two compact range appends and no per-element append:\n%s", text)
	}
	if strings.Count(text, "dynamic-array.grow-backing") != 4 {
		t.Fatalf("the empty segment must not grow:\n%s", text)
	}

	// Corrupt the ensured count: capacity for 1 value no longer covers 9.
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				if op := &block.Operations[index]; op.Kind == OpDynamicArrayHasCapacity && op.Integer.Cmp(big.NewInt(9)) == 0 {
					op.Integer = big.NewInt(1)
				}
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "covers another count") {
		t.Fatalf("mismatched ensured count accepted: %v", err)
	}
}
