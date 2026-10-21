package secmlir

import (
	"fmt"

	"sec/internal/ir/semantic"
)

// emitTarget preserves the complete typed SSA argument list on a CFG edge.
// Standard cf syntax groups operand names before the corresponding type list;
// per-operand type annotations are block-header syntax, not successor syntax.
// Rules: rules/compiler/semantic_ir.md — §66;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–86,141.
func (e *emitter) emitTarget(target semantic.BranchTarget, values map[semantic.ValueID]semantic.Value) error {
	fmt.Fprintf(&e.out, "^bb%d", target.Block)
	if len(target.Arguments) == 0 {
		return nil
	}
	e.out.WriteByte('(')
	for i, id := range target.Arguments {
		if i > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(valueName(id))
	}
	e.out.WriteString(" : ")
	if err := e.emitOperandTypes(target.Arguments, values); err != nil {
		return err
	}
	e.out.WriteByte(')')
	return nil
}
