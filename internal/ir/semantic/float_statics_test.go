package semantic

import (
	"strings"
	"testing"
)

// P20 §§ 74, 76: float statics, also as aggregate fields, carry the
// canonical bit pattern of their binary format (0.1 rounded once to
// binary32 is 0x3dcccccd; -0.0 keeps its sign bit).
func TestFloatStaticInitializers(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Point struct {
    x: float32,
    y: float64,
}

let Tiny: float32 := 0.1
let Negative: float64 := -0.0
let Origin := Point { x: 1.5, y: 2.0 }

fn Read() float32 {
    return Tiny + Origin.x
}

fn Sign() float64 {
    return Negative + Origin.y
}
`, 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"float.bits=0x3dcccccd/32", "float.bits=0x8000000000000000/64", "{float.bits=0x3fc00000/32, float.bits=0x4000000000000000/64}"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

// P20 § 28: a NaN static stores the deterministic canonical quiet NaN (the
// pattern T.NaN lowers to), an infinity its exact pattern.
func TestNonFiniteFloatStatics(t *testing.T) {
	module, err := analyzedModule(t, `module main

let Quiet: float32 := float32.NaN
let Invalid: float64 := float64.Infinity - float64.Infinity
let Huge: float64 := float64.Infinity

fn Read() float64 {
    return Invalid + Huge + float64(Quiet)
}
`, 15)
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"float.bits=0x7fc00000/32", "float.bits=0x7ff8000000000000/64", "float.bits=0x7ff0000000000000/64"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}
