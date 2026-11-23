package semantic

import (
	"os"
	"testing"
)

// The try discriminator and success-bool test remain separate, and the
// fallible operand is built once on the condition path, including recovery.
// Rules: rules/control-flow/flowcontrol_while.md — §§4–7,14,29;
// rules/compiler/semantic_ir.md — §§66–67.
func TestWhileTryConditionCFG(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/control_flow/while_conditions/effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Propagate", "Recover", "SkipTry"} {
		fn := loopFunction(t, module, name)
		counts := map[OpKind]int{}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				counts[op.Kind]++
			}
		}
		calls := 1
		if name == "SkipTry" {
			calls = 2
		}
		if len(fn.Loops) != 1 || counts[OpDirectCall] != calls || counts[OpResultIsErr] != 1 || counts[OpResultUnwrapOk] != 1 {
			t.Fatalf("%s: unexpected condition CFG: %v\n%s", name, counts, Format(module))
		}
		if name == "Propagate" && loopEdges(fn)[fn.Loops[0].ID][LoopEdgeContinue] != 1 {
			t.Fatal("continue does not target the condition")
		}
	}
}
