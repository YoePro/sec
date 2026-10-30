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

// emitRegisterConversion writes the MD-110 conversions: fits_bits names its
// destination with the register_type TypeAttr, and from_bits transports every
// Semantic IR proof basis (constant, source-domain, guarded, bit-extraction).
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2 and its 2026-10-30 amendment; MD-114 §8.
func (e *emitter) emitRegisterConversion(op semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResults(op); err != nil {
		return err
	}
	switch op.Kind {
	case semantic.OpRegisterValueFitsBits:
		register, err := e.typeText(op.RegisterConversion.Register)
		if err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.register.fits_bits\"(%s) <{register_type = %s}>", valueName(op.Operands[0]), register)
	case semantic.OpRegisterValueFromBits:
		fmt.Fprintf(&e.out, "\"sec.register.from_bits\"(%s) {sec.register_fit_proof = {basis = %s}}", valueName(op.Operands[0]), mlirString(op.RegisterConversion.Basis))
	default:
		fmt.Fprintf(&e.out, "\"sec.register.to_bits\"(%s)", valueName(op.Operands[0]))
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

// emitBitExtract writes int.bit-extract as sec.int.bit_extract with its
// width; the result type is Sema's smallest unsigned carrier.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2 amendment; semantic_ir.md § 59.
func (e *emitter) emitBitExtract(op semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if err := e.emitResults(op); err != nil {
		return err
	}
	fmt.Fprintf(&e.out, "\"sec.int.bit_extract\"(%s) <{width = %s : i32}> : (", valueName(op.Operands[0]), op.Integer.String())
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
