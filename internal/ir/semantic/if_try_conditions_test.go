package semantic

import (
	"os"
	"testing"
)

// TestIfTryBooleanConditionsLowerOnce checks Result error branching separately
// from the bool-success branch; recovery merges a bool and evaluates Probe once.
// Rules: rules/control-flow/flowcontrol_if.md — §§11,27;
// rules/errors/errorhandling.md — try conditions and local handlers;
// rules/compiler/semantic_ir.md — typed value-producing control flow.
func TestIfTryBooleanConditionsLowerOnce(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/control_flow/if_try/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 16)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, function := range module.Functions {
		switch function.Name {
		case "Propagate", "RecoverFalse", "RecoverTrue", "Negated":
		default:
			continue
		}
		found++
		counts := map[OpKind]int{}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
			}
		}
		if counts[OpDirectCall] != 1 || counts[OpResultIsErr] != 1 || counts[OpResultUnwrapOk] != 1 || counts[OpCondBranch] != 2 {
			t.Fatalf("%s: unexpected try/if CFG %v\n%s", function.Name, counts, Format(module))
		}
	}
	if found != 4 {
		t.Fatalf("checked %d functions, want 4", found)
	}
}
