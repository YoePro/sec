package secmlir

import (
	"fmt"

	"sec/internal/ir/semantic"
)

// emitBooleanNot expands semantic bool.not into existing bool constants and a
// typed CFG merge. It consumes its operand SSA value once, preserves the result
// identity and source location, and never introduces integer truthiness or a
// new Sec dialect operation. Result-based names are unique within the function.
// Rules: rules/foundations/operators.md — "Logical negation", "Logical operators";
// rules/compiler/semantic_ir.md — §67(2) (typed value-producing merges).
func (e *emitter) emitBooleanNot(operation semantic.Operation) error {
	if len(operation.Operands) != 1 || len(operation.Results) != 1 {
		return fmt.Errorf("invalid %s", operation.Kind)
	}
	id := operation.Results[0].ID
	for index, value := range []bool{false, true} {
		if index != 0 {
			e.out.WriteString("    ")
		}
		fmt.Fprintf(&e.out, "%%bool_not_%t_%d = \"sec.const.bool\"() <{value = %t}> ", value, id, value)
		e.emitOperationAttributes(operation, true)
		e.out.WriteString(": () -> i1")
		e.emitLocation(operation.Location)
		e.out.WriteByte('\n')
	}
	fmt.Fprintf(&e.out, "    cf.cond_br %s, ^bool_not_merge_%d(%%bool_not_false_%d : i1), ^bool_not_merge_%d(%%bool_not_true_%d : i1) ", valueName(operation.Operands[0]), id, id, id, id)
	e.emitOperationAttributes(operation, true)
	e.emitLocation(operation.Location)
	fmt.Fprintf(&e.out, "\n  ^bool_not_merge_%d(%s: i1):\n", id, valueName(id))
	return nil
}
