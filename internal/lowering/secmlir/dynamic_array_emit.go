package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

// dynamicArrayMnemonics maps the verified P18 Semantic IR operations to their
// schema-14 Sec MLIR operations.
var dynamicArrayMnemonics = map[semantic.OpKind]string{
	semantic.OpDynamicArrayEmpty:               "sec.dynamic_array.empty",
	semantic.OpDynamicArrayAllocate:            "sec.dynamic_array.allocate",
	semantic.OpDynamicArrayLength:              "sec.dynamic_array.len",
	semantic.OpDynamicArrayHasBacking:          "sec.dynamic_array.has_backing",
	semantic.OpDynamicArrayHasCapacity:         "sec.dynamic_array.has_capacity",
	semantic.OpDynamicArrayIndexInBounds:       "sec.dynamic_array.index_in_bounds",
	semantic.OpDynamicArrayElementPlace:        "sec.dynamic_array.element_place",
	semantic.OpDynamicArrayAppendPrepared:      "sec.dynamic_array.append_prepared",
	semantic.OpDynamicArrayAppendRangePrepared: "sec.dynamic_array.append_range_prepared",
	semantic.OpDynamicArrayGrowBacking:         "sec.dynamic_array.grow_backing",
	semantic.OpDynamicArrayClear:               "sec.dynamic_array.clear",
	semantic.OpDynamicArrayRemoveAt:            "sec.dynamic_array.remove_at",
	semantic.OpDynamicArrayPayloadSize:         "sec.dynamic_array.payload_size",
	semantic.OpStorageEstablishDomain:          "sec.storage.establish_domain",
	semantic.OpStorageAdvanceEpoch:             "sec.storage.advance_epoch",
	semantic.OpStorageEndDomain:                "sec.storage.end_domain",
	semantic.OpStorageReclaim:                  "sec.storage.reclaim",
}

// emitDynamicArray maps verified P18 operations directly to schema 14,
// preserving the owner Place operand, the resolved backing facts and the
// guarded element Place's backing-domain dependency. It selects no allocator,
// descriptor layout or address; the protocol CFG stays as built.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 21–31, 58–67, 81–96.
func (e *emitter) emitDynamicArray(function *semantic.Function, op semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if e.schemaVersion < 14 {
		return &UnsupportedLoweringError{Feature: "dynamic-array and storage-transition operations require schema 14", Function: function.ID}
	}
	mnemonic, ok := dynamicArrayMnemonics[op.Kind]
	if !ok {
		return &UnsupportedLoweringError{Feature: string(op.Kind), Function: function.ID}
	}
	var properties []string
	switch op.Kind {
	case semantic.OpDynamicArrayAllocate, semantic.OpDynamicArrayHasCapacity, semantic.OpDynamicArrayGrowBacking, semantic.OpDynamicArrayAppendRangePrepared:
		if op.Integer == nil || op.Integer.Sign() <= 0 {
			return fmt.Errorf("%s in %s has no positive count", op.Kind, function.ID)
		}
		properties = append(properties, "count = "+mlirString(op.Integer.String()))
	case semantic.OpDynamicArrayIndexInBounds:
		properties = append(properties, fmt.Sprintf("index_signed = %t", op.ArrayIndexSigned))
	case semantic.OpDynamicArrayPayloadSize:
		if op.Integer == nil || op.Integer.Sign() < 0 {
			return fmt.Errorf("dynamic-array.payload-size in %s has no stride", function.ID)
		}
		properties = append(properties, "stride = "+mlirString(op.Integer.String()))
	case semantic.OpDynamicArrayClear:
		if op.Ownership != nil && op.Ownership.DestructionPlan != "" {
			properties = append(properties, "destruction_plan = "+mlirString(op.Ownership.DestructionPlan))
		}
	case semantic.OpDynamicArrayRemoveAt:
		properties = append(properties, fmt.Sprintf("index_signed = %t", op.ArrayIndexSigned),
			"bounds_kind = "+mlirString(string(op.ArrayCheckKind)), "bounds_proof = "+mlirString(string(op.ArrayProofKind)))
	case semantic.OpDynamicArrayElementPlace:
		if op.Place == nil || op.Place.BackingDomain == 0 {
			return fmt.Errorf("dynamic-array.element-place in %s has no backing-domain dependency", function.ID)
		}
		properties = append(properties, fmt.Sprintf("index_signed = %t", op.ArrayIndexSigned),
			"bounds_kind = "+mlirString(string(op.ArrayCheckKind)), "bounds_proof = "+mlirString(string(op.ArrayProofKind)),
			fmt.Sprintf("backing_domain = %d : i32", op.Place.BackingDomain))
	}
	properties = append(properties, backingProperties(op.Backing)...)
	if len(op.Results) != 0 {
		if err := e.emitResults(op); err != nil {
			return err
		}
	}
	operands := make([]string, len(op.Operands))
	for index, operand := range op.Operands {
		operands[index] = valueName(operand)
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s) ", mnemonic, strings.Join(operands, ", "))
	if len(properties) != 0 {
		fmt.Fprintf(&e.out, "<{%s}> ", strings.Join(properties, ", "))
	}
	if extras := operationAttributes(op, false); len(extras) != 0 {
		fmt.Fprintf(&e.out, "{%s} ", strings.Join(extras, ", "))
	}
	return e.finishTypedOperation(op, values)
}

// backingProperties spells resolved backing facts as typed properties; an
// absent fact is omitted rather than invented.
func backingProperties(backing *semantic.BackingMetadata) []string {
	if backing == nil {
		return nil
	}
	var properties []string
	identity := func(name string, value uint32) {
		if value != 0 {
			properties = append(properties, fmt.Sprintf("%s = %d : i32", name, value))
		}
	}
	text := func(name, value string) {
		if value != "" {
			properties = append(properties, name+" = "+mlirString(value))
		}
	}
	identity("domain", backing.Domain)
	text("transition", backing.Transition)
	identity("allocation", backing.Allocation)
	identity("allocation_context", backing.Context)
	text("origin", backing.Origin)
	text("relation", backing.Relation)
	text("authority", backing.Authority)
	identity("reclamation", backing.Reclamation)
	text("stability", backing.Stability)
	text("space", backing.Space)
	return properties
}
