package semantic

import (
	"strings"
	"testing"
)

// package20.md §§ 28–36, 63, 66–68: float core methods and associated
// properties build the total float.* operations; integer Min/Max/Bits are
// exact constants; Clamp is float.clamp-checked building Result[T, RangeError].
func TestFloatCoreOperations(t *testing.T) {
	module, err := buildPackage15Source(t, `
module main

fn Core(x: float64, y: float32) float64 {
    let n := x.IsNaN()
    let s := y.IsSubnormal()
    let r := y.Ceiling()
    let i := int16.Min
    let k := int64.Bits
    discard n
    discard s
    discard r
    discard i
    discard k
    return x.Abs().Min(float64.Epsilon).Max(x.Round())
}

fn main() int {
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"float.classify nan", "float.classify subnormal", "float.round ceiling", "float.round nearest-even",
		"float.abs", "float.minimum", "float.maximum", "float.property epsilon", "const.int -32768", "const.int 64"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	for index := range module.Functions[0].Blocks[0].Operations {
		op := &module.Functions[0].Blocks[0].Operations[index]
		if op.Kind == OpFloatRound {
			op.FloatCore = "half-up"
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "invalid float.round") {
		t.Fatalf("an unknown rounding kind must be rejected, got %v", err)
	}

	clamped, err := buildPackage15Source(t, `
module main

fn Bounded(x: float64) Result[float64, RangeError] {
    let v := try x.Clamp(0.0, 1.0)
    return Ok(v)
}

fn main() int {
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(clamped); err != nil {
		t.Fatal(err)
	}
	clampText := Format(clamped)
	for _, want := range []string{"float.clamp-checked", "enum.constant", "result.err", "result.ok"} {
		if !strings.Contains(clampText, want) {
			t.Fatalf("missing %q in:\n%s", want, clampText)
		}
	}
	// The clamped value is consumed only on the success edge.
	for _, fn := range clamped.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				op := &block.Operations[index]
				if op.Kind == OpCondBranch && len(op.Successors) == 2 && index > 0 && block.Operations[index-1].Kind == OpFloatClampChecked {
					op.Successors[0], op.Successors[1] = op.Successors[1], op.Successors[0]
				}
			}
		}
	}
	if err := Verify(clamped); err == nil || !strings.Contains(err.Error(), "candidate") {
		t.Fatalf("a clamped value consumed on the failure edge must be rejected, got %v", err)
	}
}
