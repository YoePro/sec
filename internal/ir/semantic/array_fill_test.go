package semantic

import (
	"os"
	"strings"
	"testing"
)

// Contextual fixed-array fill(value) evaluates the value once and copies it
// into every element with one array.fill; a zero-length target is a fill of
// no elements. The verifier rejects a mistyped value.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill".
func TestFixedArrayFillSemanticIR(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/ir/test_fixed_array_fill.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := hostedModule(t, string(source)+"\nfn Empty() int[0] {\n    return fill(1)\n}\n", 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{`array.fill %`, `length="4"`, `length="3"`, `length="0"`, "call.direct"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "array.fill %") != 3 || strings.Count(text, `func "main::Seed()"`) != 1 {
		t.Fatalf("want three fills and Seed evaluated through one call per fill:\n%s", text)
	}
	fn := loopFunction(t, module, "Build")
	values := map[ValueID]Value{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, result := range op.Results {
				values[result.ID] = result
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpArrayFill {
				continue
			}
			if countOperations(fn, OpDirectCall) != 1 {
				t.Fatalf("fill must evaluate its value once:\n%s", text)
			}
			wrong := op
			wrong.ArrayLength = "5"
			if err := verifyArrayFill(module, wrong, values); err == nil {
				t.Fatal("verifier accepted a length mismatch")
			}
		}
	}
}
