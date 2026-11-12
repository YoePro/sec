package semantic

import "testing"

// A shift count keeps its own integer type and the result keeps the left
// operand's type, so a named integer shifts by a literal, a built-in or a
// named count, in infix and compound form, and the module verifies.
//
// Rules:
//   - rules/foundations/operators.md — § 37(2)–(3) "Shift operators", "Compound assignment"
//   - rules/compiler/semantic_ir.md — § 58 "Arithmetic"
func TestNamedIntegerShiftsLowerAndVerify(t *testing.T) {
	_, module, err := buildWithPlan(t, `
module main

type Count uint8

fn Shift(count: Count, by: Count, plain: uint8) Count {
    let other := count >> 7
    let named := other << by
    let mixed := plain >> by
    let mut total: Count := named
    total >>= 7
    total <<= by
    return total
}
`, 64)
	if err != nil {
		t.Fatal(err)
	}
	shifts := 0
	for _, block := range functionNamed(t, module, "Shift").Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpIntShiftChecked {
				shifts++
			}
		}
	}
	if shifts != 5 {
		t.Fatalf("int.shift-checked operations = %d, want 5", shifts)
	}
}
