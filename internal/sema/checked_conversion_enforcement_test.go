package sema

import (
	"strings"
	"testing"
)

// types.md "Explicit conversions" and MD-056: a Sec-only scalar conversion
// that may fail requires try; bit[N](value) is the deliberate low-bit
// extraction, and a conversion proven exact (constant, source domain, plan
// width) needs nothing.
func TestSecScalarConversionsThatMayFailRequireTry(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Bad(wide: int, count: uint, scalar: uint) int8 {
    let a := uint(wide)
    let b := int(count)
    let c := rune(scalar)
    discard a
    discard b
    discard c
    return int8(wide)
}

fn Good(small: int8, wide: int64, count: uint, scalar: uint) uint64 {
    let a := int(small)
    let b := int8(100)
    let c := bit[64](wide)
    let d := try int(count) {
        Err(_) => 0
    }
    let e := try rune(scalar) {
        Err(_) => 65533r
    }
    discard a
    discard b
    discard d
    discard e
    return c
}
`)
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	for _, want := range []string{
		"converting int to uint may fail with ConversionError.OutOfRange and requires try",
		"converting uint to int may fail with ConversionError.OutOfRange and requires try",
		"converting uint to rune may fail with ConversionError.InvalidScalar and requires try",
		"converting int to int8 may fail with ConversionError.OutOfRange and requires try",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if len(errors) != 4 {
		t.Fatalf("only the four unprotected conversions in Bad are errors, got:\n%s", joined)
	}
}
