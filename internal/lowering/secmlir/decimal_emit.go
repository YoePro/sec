package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

// emitDecimalOperation writes a checked decimal operator with the Sema plan
// as typed attributes: kind, coefficient width, preferred-scale rule, the
// nearest-exact scale policy, nearest-even quotient rounding or truncating
// remainder, and the canonical ArithmeticError failure reasons. Results are
// (candidate, failed, reason) like checked integer arithmetic.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v17.md — §§ 3, 6;
// rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 12(4)–12(5).
func (e *emitter) emitDecimalOperation(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	facts := operation.DecimalOperation
	if facts == nil {
		return fmt.Errorf("%s without its decimal plan", operation.Kind)
	}
	name, kind := "sec.decimal.binary_checked", string(facts.Kind)
	if operation.Kind == semantic.OpDecimalNegChecked {
		name, kind = "sec.decimal.unary", "neg"
	}
	if err := e.emitResults(operation); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"%s\"(", name)
	for index, operand := range operation.Operands {
		if index > 0 {
			e.out.WriteString(", ")
		}
		e.out.WriteString(valueName(operand))
	}
	failures := make([]string, len(facts.Failures))
	for index, reason := range facts.Failures {
		failures[index] = mlirString(string(reason))
	}
	fmt.Fprintf(&e.out, ") <{coefficient_bits = %d : i32, failures = [%s], kind = %s, preferred_scale = %s",
		facts.CoefficientBits, strings.Join(failures, ", "), mlirString(kind), mlirString(facts.PreferredScale))
	if facts.RoundsInexactQuotient {
		e.out.WriteString(`, quotient = "nearest-even"`)
	}
	if facts.TruncatingRemainder {
		e.out.WriteString(`, remainder = "truncating"`)
	}
	e.out.WriteString(`, scale_policy = "nearest-exact"}>`)
	if operation.TryHandlerKind != "" || operation.MatchID != 0 {
		e.out.WriteByte(' ')
		e.emitOperationAttributes(operation, false)
	}
	e.out.WriteString(" : (")
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	if err := e.emitResultTypes(operation.Results); err != nil {
		return err
	}
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}
