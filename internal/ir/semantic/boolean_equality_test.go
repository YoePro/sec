package semantic

import (
	"os"
	"testing"
)

// Rules: rules/foundations/operators.md — "Boolean equality", "Evaluation order".
func TestBooleanEqualityBuildsEagerTypedFlow(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/boolean_equality/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []uint8{7, 12, 14} {
		module, err := analyzedModule(t, string(data), pkg)
		if err != nil {
			t.Fatalf("package %d: %v", pkg, err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Equal", "Unequal", "BothEqual", "BothUnequal", "ConstantEqual", "ConstantUnequal", "Nested"} {
			fn := loopFunction(t, module, name)
			entry := fn.Blocks[fn.Entry]
			branch := entry.Operations[len(entry.Operations)-1]
			if branch.Kind != OpCondBranch || (branch.Operator != "==" && branch.Operator != "!=") {
				t.Fatalf("%s: missing resolved equality branch", name)
			}
			if len(fn.Blocks) != 3 || countOperations(fn, OpCondBranch) != 2 || countOperations(fn, OpIntCompare) != 0 || countOperations(fn, OpBoolNot) != 0 {
				t.Fatalf("%s: incorrect typed boolean CFG\n%s", name, Format(module))
			}
			var calls []FunctionID
			for _, op := range entry.Operations[:len(entry.Operations)-1] {
				if op.Kind == OpDirectCall {
					calls = append(calls, op.Callee)
				}
			}
			switch name {
			case "BothEqual", "BothUnequal":
				left, right := loopFunction(t, module, "Left"), loopFunction(t, module, "Right")
				if len(calls) != 2 || calls[0] != left.ID || calls[1] != right.ID || countOperations(fn, OpDirectCall) != 2 {
					t.Fatalf("%s: operands were not evaluated exactly once, left first: %v", name, calls)
				}
			case "ConstantEqual", "ConstantUnequal":
				if len(calls) != 1 || calls[0] != loopFunction(t, module, "Right").ID || countOperations(fn, OpDirectCall) != 1 {
					t.Fatalf("%s: constant left operand elided the required right call", name)
				}
			}
		}
	}
}
