package semantic

import (
	"strings"
	"testing"
)

// sliceFillSource fills a mutable slice parameter and calls it with an
// explicit inline mutable borrow of a fixed array.
const sliceFillSource = "module main\n\nfn Reset(values: ref mut int[]) void {\n    values.Fill(7)\n}\n\nfn main() int {\n    let mut data: int[3] := [1, 2, 3]\n    Reset(ref mut data[..])\n    return data[0]\n}\n"

// Mutable-slice Fill(value) evaluates the value once and lowers to one
// slice.fill through the slice holder; the verifier rejects a shared slice
// and a mistyped value.
// Rules: rules/collections/collections.md — § 7.9.
func TestSliceFillSemanticIR(t *testing.T) {
	module, err := hostedModule(t, sliceFillSource, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if strings.Count(text, "slice.fill %") != 1 {
		t.Fatalf("want one slice.fill:\n%s", text)
	}
	fn := loopFunction(t, module, "Reset")
	values := map[ValueID]Value{}
	for _, parameter := range fn.Parameters {
		values[parameter.Value.ID] = parameter.Value
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, result := range op.Results {
				values[result.ID] = result
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpSliceFill {
				continue
			}
			swapped := op
			swapped.Operands = []ValueID{op.Operands[1], op.Operands[0]}
			if err := verifySliceFillOperation(module, swapped, values); err == nil {
				t.Fatal("verifier accepted a non-slice receiver")
			}
		}
	}
}
