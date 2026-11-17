package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

// emitUnitDimensionOperation writes the Sec MLIR form of a UnitDimension value
// operation (MD-062 §§ 7(12), 12(8)): a constant lists its canonical axes, and
// comparison, AxisCount and IsDimensionless read operands of
// !sec.unit_dimension.
func (e *emitter) emitUnitDimensionOperation(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if e.schemaVersion < 13 {
		return &UnsupportedLoweringError{Feature: "UnitDimension values before schema 13", SchemaVersion: e.schemaVersion}
	}
	switch operation.Kind {
	case semantic.OpUnitDimensionConstant:
		if operation.UnitDimension == nil {
			return fmt.Errorf("unit-dimension.constant without its dimension")
		}
		axes := make([]string, len(operation.UnitDimension.Axes))
		for index, axis := range operation.UnitDimension.Axes {
			axes[index] = fmt.Sprintf("{axis = %s, exponent = %d : i64}", mlirString(axis.Axis), axis.Exponent)
		}
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.unit_dimension.constant\"() <{axes = [%s]}> : () -> ", strings.Join(axes, ", "))
		return e.finishResultOperation(operation)
	case semantic.OpUnitDimensionCompare:
		if len(operation.Operands) != 2 || len(operation.Results) != 1 {
			return fmt.Errorf("invalid %s", operation.Kind)
		}
		predicate := "eq"
		if operation.Operator == "!=" {
			predicate = "ne"
		}
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.unit_dimension.compare\"(%s, %s) <{predicate = %s}> : (!sec.unit_dimension, !sec.unit_dimension) -> i1", valueName(operation.Operands[0]), valueName(operation.Operands[1]), mlirString(predicate))
		e.emitLocation(operation.Location)
		e.out.WriteByte('\n')
		return nil
	case semantic.OpUnitDimensionAxisCount:
		return e.emitUnarySemanticOperation(operation, "sec.unit_dimension.axis_count", values)
	default:
		return e.emitUnarySemanticOperation(operation, "sec.unit_dimension.is_dimensionless", values)
	}
}
