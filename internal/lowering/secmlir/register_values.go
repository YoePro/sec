package secmlir

import (
	"fmt"
	"sec/internal/ir/semantic"
	"strings"
)

// emitRegisterTable serializes the verified canonical first-use register table
// without recreating layout, unit identities or effective access/shadow policy.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §§2–3,10(5)).
func (e *emitter) emitRegisterTable() error {
	if len(e.module.Registers) == 0 {
		return nil
	}
	e.out.WriteString("sec.registers = [")
	for index, definition := range e.module.Registers {
		if index > 0 {
			e.out.WriteString(", ")
		}
		typ, _ := e.module.Types.Lookup(definition.TypeID)
		fmt.Fprintf(&e.out, "#sec.register_def<identity = %s, width = %d, order = %s", mlirString(typ.Identity), definition.Width, mlirString(definition.AllocationOrder))
		if definition.ByteOrder != "" {
			fmt.Fprintf(&e.out, ", byte_order = %s", mlirString(definition.ByteOrder))
		}
		e.out.WriteString(", fields = [")
		for index, field := range definition.Fields {
			if field.Unit != "" {
				return &UnsupportedLoweringError{Feature: "register unit-qualified field representation"}
			}
			if index > 0 {
				e.out.WriteString(", ")
			}
			carrier, err := e.typeText(field.Type)
			if err != nil {
				return err
			}
			fmt.Fprintf(&e.out, "#sec.register_field<ordinal = %d, name = %s, offset = %d, width = %d, type = %s, access = %s, shadow = %t, reserved = %t>", field.ID, mlirString(field.Name), field.BitOffset, field.Width, carrier, mlirString(field.Access), field.Shadow == "shadow", field.Reserved)
		}
		e.out.WriteString("]>")
	}
	e.out.WriteString("], ")
	return nil
}

// emitRegisterValue consumes resolved field IDs/proofs. A numeric proof must
// already be tied to SSA by Semantic IR verification before any bytes escape.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §§5–7).
func (e *emitter) emitRegisterValue(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResults(operation); err != nil {
		return err
	}
	name := "sec.register.extract"
	if operation.Kind == semantic.OpRegisterValueUpdate {
		name = "sec.register.update"
	}
	operands := make([]string, len(operation.Operands))
	for i, value := range operation.Operands {
		operands[i] = valueName(value)
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s) {sec.field_path = [%d : i32]", name, strings.Join(operands, ", "), operation.RegisterField)
	if operation.Kind == semantic.OpRegisterValueUpdate {
		proof := operation.RegisterWrite
		fmt.Fprintf(&e.out, ", sec.write_proof = #sec.register_write_proof<basis = %s", mlirString(proof.Basis))
		if proof.Basis != "typed" {
			fmt.Fprintf(&e.out, ", low = %s, high = %s", mlirString(proof.Low.String()), mlirString(proof.High.String()))
		}
		e.out.WriteByte('>')
	}
	e.out.WriteString("} : (")
	if err := e.emitOperandTypes(operation.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	result, err := e.typeText(operation.Results[0].Type)
	if err != nil {
		return err
	}
	e.out.WriteString(result)
	e.emitLocation(operation.Location)
	e.out.WriteByte('\n')
	return nil
}

// emitRegisterConversion preserves the already admitted MD-110 proof families;
// unavailable guarded/bit-extraction transport fails without inventing syntax.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2; MD-114 §8.
func (e *emitter) emitRegisterConversion(op semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if op.Kind == semantic.OpRegisterValueFitsBits || op.RegisterConversion.Basis == "guarded" {
		return &UnsupportedLoweringError{Feature: "MD-110 fits_bits destination-identity transport"}
	}
	if op.RegisterConversion.Basis == "bit-extraction" {
		return &UnsupportedLoweringError{Feature: "MD-110 bit-extraction proof transport"}
	}
	if err := e.emitResults(op); err != nil {
		return err
	}
	name := "sec.register.from_bits"
	if op.Kind == semantic.OpRegisterValueToBits {
		name = "sec.register.to_bits"
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s)", name, valueName(op.Operands[0]))
	if op.Kind == semantic.OpRegisterValueFromBits {
		fmt.Fprintf(&e.out, " {sec.register_fit_proof = {basis = %s}}", mlirString(op.RegisterConversion.Basis))
	}
	e.out.WriteString(" : (")
	if err := e.emitOperandTypes(op.Operands, values); err != nil {
		return err
	}
	e.out.WriteString(") -> ")
	result, err := e.typeText(op.Results[0].Type)
	if err != nil {
		return err
	}
	e.out.WriteString(result)
	e.emitLocation(op.Location)
	e.out.WriteByte('\n')
	return nil
}

// validateRegisterConversionTransport rejects the two unresolved MD-110 paths
// before any operation is serialized, even when a preceding low-bit extraction
// would otherwise obscure the actual missing register-proof contract.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2; MD-114 §8(4).
func validateRegisterConversionTransport(module *semantic.Module) error {
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.RegisterConversion == nil {
					continue
				}
				if op.Kind == semantic.OpRegisterValueFitsBits || op.RegisterConversion.Basis == "guarded" {
					return &UnsupportedLoweringError{Feature: "MD-110 fits_bits destination-identity transport", Function: fn.ID}
				}
				if op.RegisterConversion.Basis == "bit-extraction" {
					return &UnsupportedLoweringError{Feature: "MD-110 bit-extraction proof transport", Function: fn.ID}
				}
			}
		}
	}
	return nil
}
