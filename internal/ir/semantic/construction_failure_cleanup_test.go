package semantic

import (
	"strings"
	"testing"
)

// A construction whose later operand fails destroys only the operands it
// already evaluated, each through its own destruction plan, and leaves: the
// failure path never builds the outer struct, union or array and never names
// the outer value's destruction plan, so no custom free of the outer type can
// run for a value that was never completed.
// Rules: rules/memory/destruction.md — §§ 7, 8, 14.2;
// rules/compiler/semantic_ir.md — construction-failure cleanup.
func TestConstructionFailureNeverManufacturesOuterValue(t *testing.T) {
	source := "module main\n\ntype Handle struct {\n    id: int,\n}\n\nimpl Handle {\n    free {\n    }\n}\n\ntype Outer struct {\n    left:  Handle,\n    right: Handle,\n}\n\nimpl Outer {\n    free {\n    }\n}\n\ntype Parcel union {\n    Boxed { inner: Handle, outer: Handle },\n    Empty,\n}\n\nfn Make(id: int) Handle {\n    return Handle { id: id }\n}\n\nfn TryMake(id: int) Result[Handle, IndexError] {\n    return Ok(Make(id))\n}\n\nfn Struct() Result[int, IndexError] {\n    let outer := Outer { left: Make(1), right: try TryMake(2) }\n    return Ok(1)\n}\n\nfn Union() Result[int, IndexError] {\n    let parcel := Parcel.Boxed { inner: Make(1), outer: try TryMake(2) }\n    return Ok(1)\n}\n\nfn Array() Result[int, IndexError] {\n    let handles := [Make(1), try TryMake(2)]\n    return Ok(1)\n}\n\nfn main() int {\n    return 0\n}\n"
	module, err := hostedModule(t, source, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for name, outer := range map[string]OpKind{"Struct": OpStructConstruct, "Union": OpUnionConstruct, "Array": OpArrayConstruct} {
		fn := loopFunction(t, module, name)
		failures := 0
		for _, block := range fn.Blocks {
			prepared := false
			for _, op := range block.Operations {
				if op.Kind == OpOwnDestroyValue && op.Ownership != nil && op.Ownership.Cause == "temporary-end" {
					prepared = true
					if op.Ownership.DestructionPlan != "destroy:main:struct:Handle" {
						t.Fatalf("%s destroys a prepared operand with plan %q", name, op.Ownership.DestructionPlan)
					}
				}
			}
			if !prepared {
				continue
			}
			failures++
			for _, op := range block.Operations {
				if op.Kind == outer && len(op.Results) == 1 {
					if typ, _ := module.Types.Lookup(op.Results[0].Type); typ.Kind != TypeResult {
						t.Fatalf("%s failure path manufactures the outer value: %+v", name, op)
					}
				}
				if op.Ownership != nil && strings.Contains(op.Ownership.DestructionPlan, "Outer") || op.Ownership != nil && strings.Contains(op.Ownership.DestructionPlan, "Parcel") {
					t.Fatalf("%s failure path names the outer destruction plan: %+v", name, op)
				}
			}
		}
		if failures != 1 {
			t.Fatalf("%s: want one failure exit destroying the prepared operand, got %d", name, failures)
		}
	}
}
