package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

// Schema-11 emission of Semantic IR Places and direct safe references
// (SEC-MLIR-P15). Places stay semantic roots and projections, references keep
// their borrow/lifetime/validity facts, and nothing selects a pointer, epoch
// field, offset or ABI representation.
//
// Facts the schema-11 operations do not carry as properties are kept as
// discardable attributes (implementation choice MD-084): a reference function
// parameter or call result names its facts in a `sec.reference` dictionary,
// a function returning a reference names its origin summary in
// `sec.return_origins`, address-taken storage carries `sec.address_taken`,
// and reference operations keep origin storages, pinning and a non-ordinary
// address space as `sec.origin_storages`, `sec.pinned` and
// `sec.reference_address_space`.
//
// Rules:
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v11.md — sections 1-28
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v11.md — sections 2-30
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — sections 59-67, 87

const invalidReferenceGenerationReason = "panic.invalid-reference-generation"

func (e *emitter) placeReferenceTypeText(typeValue semantic.Type) (string, error) {
	element, err := e.typeText(typeValue.Element)
	if err != nil {
		return "", err
	}
	switch {
	case typeValue.Kind == semantic.TypePlace && typeValue.Mutable:
		return fmt.Sprintf("!sec.place<%s, \"rw\">", element), nil
	case typeValue.Kind == semantic.TypePlace:
		return fmt.Sprintf("!sec.place<%s, \"ro\">", element), nil
	case typeValue.Mutable:
		return fmt.Sprintf("!sec.ref_mut<%s>", element), nil
	}
	return fmt.Sprintf("!sec.ref<%s>", element), nil
}

// referenceProperties prints the schema-11 reference metadata properties.
// Copies and moves keep their source borrow, so only reborrows name one.
func referenceProperties(ref *semantic.ReferenceMetadata, derived bool) string {
	var out strings.Builder
	fmt.Fprintf(&out, "borrow_id = %d : i32, lifetime_id = %d : i32, validity_policy = %s", ref.Borrow, ref.Lifetime, mlirString(string(ref.Validity)))
	if ref.ProofProvenance != "" {
		fmt.Fprintf(&out, ", validity_proof = %s", mlirString(ref.ProofProvenance))
	}
	if ref.Validity == semantic.ReferenceDynamicEpoch {
		fmt.Fprintf(&out, ", invalidation_domain = %d : i32, epoch_dependency = %d : i32", ref.InvalidationDomain, ref.EpochDependency)
	}
	fmt.Fprintf(&out, ", address_stability = %s, provenance = %s", mlirString(ref.AddressStability), mlirString(ref.Provenance))
	if derived {
		fmt.Fprintf(&out, ", source_borrow = %d : i32", ref.SourceBorrow)
	}
	return out.String()
}

func originStoragesText(origins []uint32) string {
	parts := make([]string, len(origins))
	for index, origin := range origins {
		parts[index] = fmt.Sprintf("%d : i32", origin)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// referenceExtras prints the reference facts outside the schema-11
// properties, each prefixed for its position.
func referenceExtras(ref *semantic.ReferenceMetadata, prefix string) []string {
	var extras []string
	if len(ref.OriginStorages) > 0 {
		extras = append(extras, fmt.Sprintf("%sorigin_storages = %s", prefix, originStoragesText(ref.OriginStorages)))
	}
	if ref.Pinned {
		extras = append(extras, prefix+"pinned = true")
	}
	if ref.AddressSpace != semantic.AddressSpaceOrdinary {
		name := "reference_address_space"
		if prefix == "" {
			name = "address_space"
		}
		extras = append(extras, fmt.Sprintf("%s%s = %s", prefix, name, mlirString(string(ref.AddressSpace))))
	}
	return extras
}

// referenceDictionary prints the complete facts of a reference that no
// schema-11 operation defines: a function parameter or a call result.
func referenceDictionary(ref *semantic.ReferenceMetadata) string {
	parts := append([]string{referenceProperties(ref, ref.SourceBorrow != 0 && ref.SourceBorrow != ref.Borrow)}, referenceExtras(ref, "")...)
	return "sec.reference = {" + strings.Join(parts, ", ") + "}"
}

func supportedAddressSpace(space semantic.AddressSpace) bool {
	return space == semantic.AddressSpaceOrdinary || space == semantic.AddressSpaceMMIO
}

// validateReferenceFacts rejects facts schema 11 cannot spell instead of
// dropping them: target-defined address spaces have no MLIR form yet.
func validateReferenceFacts(ref *semantic.ReferenceMetadata, function semantic.FunctionID) error {
	if ref == nil {
		return nil
	}
	if !supportedAddressSpace(ref.AddressSpace) {
		return &UnsupportedLoweringError{Feature: "reference in address space " + string(ref.AddressSpace), Function: function}
	}
	return nil
}

// returnOriginsText prints the returned-reference origin summary with the
// canonical Semantic IR origin spelling: param#N, receiver or static@N,
// followed by .field#N, [K], [*] and schema-12 [A..<B]/[?] steps.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.7,v12.34 (MD-084/115).
func returnOriginsText(origins []semantic.ReturnOrigin) string {
	parts := make([]string, 0, len(origins))
	for _, origin := range origins {
		var text strings.Builder
		switch origin.Kind {
		case semantic.ReturnOriginReceiver:
			text.WriteString("receiver")
		case semantic.ReturnOriginStatic:
			fmt.Fprintf(&text, "static@%d", origin.Static)
		default:
			fmt.Fprintf(&text, "param#%d", origin.Parameter)
		}
		for _, step := range origin.Path {
			switch {
			case step.Kind == semantic.ReturnOriginSlice:
				if step.UnknownRange {
					text.WriteString("[?]")
				} else {
					fmt.Fprintf(&text, "[%s..<%s]", step.Start, step.EndExclusive)
				}
			case step.Kind == semantic.ReturnOriginField:
				fmt.Fprintf(&text, ".field#%d", step.Field)
			case step.Index != nil:
				fmt.Fprintf(&text, "[%s]", step.Index.String())
			default:
				text.WriteString("[*]")
			}
		}
		parts = append(parts, mlirString(text.String()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (e *emitter) emitPlaceReference(function *semantic.Function, operation semantic.Operation, values map[semantic.ValueID]semantic.Value, storages map[semantic.StorageID]semantic.Storage) error {
	if err := validateReferenceFacts(operation.Reference, function.ID); err != nil {
		return err
	}
	if place := operation.Place; place != nil && !supportedAddressSpace(place.AddressSpace) {
		return &UnsupportedLoweringError{Feature: "Place in address space " + string(place.AddressSpace), Function: function.ID}
	}
	rootProperties := func() (string, error) {
		place := operation.Place
		if place == nil {
			return "", fmt.Errorf("%s requires Place root metadata", operation.Kind)
		}
		return fmt.Sprintf("place_root_id = %d : i32, storage_identity = %d : i32, address_space = %s", place.Root, place.Storage, mlirString(string(place.AddressSpace))), nil
	}
	referenceOperation := func(name string, derived bool) error {
		if operation.Reference == nil {
			return fmt.Errorf("%s requires reference metadata", operation.Kind)
		}
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"%s\"(%s) <{%s}> ", name, valueName(operation.Operands[0]), referenceProperties(operation.Reference, derived))
		if extras := referenceExtras(operation.Reference, "sec."); len(extras) > 0 {
			fmt.Fprintf(&e.out, "{%s} ", strings.Join(extras, ", "))
		}
		return e.finishTypedOperation(operation, values)
	}
	switch operation.Kind {
	case semantic.OpPlaceValueRoot:
		properties, err := rootProperties()
		if err != nil {
			return err
		}
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.value\"(%s) <{%s}> ", valueName(operation.Operands[0]), properties)
		return e.finishTypedOperation(operation, values)
	case semantic.OpPlaceStorageRoot:
		storage, ok := storages[operation.Storage]
		if !ok {
			return fmt.Errorf("missing storage %d", operation.Storage)
		}
		properties, err := rootProperties()
		if err != nil {
			return err
		}
		elementType, err := e.typeText(storage.Type)
		if err != nil {
			return err
		}
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.storage\"(%s) <{%s}> : (!sec.storage<%s>) -> ", storageName(storage.ID), properties, elementType)
		return e.finishResultOperation(operation)
	case semantic.OpPlaceStaticRoot:
		// MD-116: a zero-operand root naming the static's sec.static.global,
		// whose explicit sec.static_id the return-origin verifier reads.
		properties, err := rootProperties()
		if err != nil {
			return err
		}
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.static\"() <{global = @%s, %s}> : () -> ", staticSymbol(operation.Static), properties)
		return e.finishResultOperation(operation)
	case semantic.OpPlaceField:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.field\"(%s) <{field = %d : i32}> ", valueName(operation.Operands[0]), operation.StructField)
		return e.finishTypedOperation(operation, values)
	case semantic.OpPlaceArrayIndexInBounds:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.array_index_in_bounds\"(%s, %s) <{index_signed = %t}> ", valueName(operation.Operands[0]), valueName(operation.Operands[1]), operation.ArrayIndexSigned)
		return e.finishTypedOperation(operation, values)
	case semantic.OpPlaceArrayElement:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.array_element\"(%s, %s) <{bounds_kind = %s, bounds_proof = %s}> ", valueName(operation.Operands[0]), valueName(operation.Operands[1]),
			mlirString(string(operation.ArrayCheckKind)), mlirString(string(operation.ArrayProofKind)))
		return e.finishTypedOperation(operation, values)
	case semantic.OpPlaceUnionPayload:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.union_payload\"(%s) <{variant = %d : i32}> ", valueName(operation.Operands[0]), operation.UnionVariant)
		return e.finishTypedOperation(operation, values)
	case semantic.OpReferenceDerefPlace:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.deref\"(%s) ", valueName(operation.Operands[0]))
		return e.finishTypedOperation(operation, values)
	case semantic.OpPlaceRead:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.place.read\"(%s) <{action = %s}> ", valueName(operation.Operands[0]), mlirString(string(semantic.ArgumentCopyTrivial)))
		return e.finishTypedOperation(operation, values)
	case semantic.OpPlaceWrite:
		fmt.Fprintf(&e.out, "\"sec.place.write\"(%s, %s) ", valueName(operation.Operands[0]), valueName(operation.Operands[1]))
		return e.finishTypedOperation(operation, values)
	case semantic.OpReferenceBorrowShared:
		return referenceOperation("sec.ref.borrow_shared", false)
	case semantic.OpReferenceBorrowMutable:
		return referenceOperation("sec.ref.borrow_mut", false)
	case semantic.OpReferenceCopyShared:
		return referenceOperation("sec.ref.copy_shared", false)
	case semantic.OpReferenceMove:
		return referenceOperation("sec.ref.move", false)
	case semantic.OpReferenceReborrowShared:
		return referenceOperation("sec.ref.reborrow_shared", true)
	case semantic.OpReferenceReborrowMutable:
		return referenceOperation("sec.ref.reborrow_mut", true)
	case semantic.OpReferenceIsValid:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.ref.is_valid\"(%s) ", valueName(operation.Operands[0]))
		return e.finishTypedOperation(operation, values)
	case semantic.OpReferenceCompare:
		if err := e.emitResult(operation); err != nil {
			return err
		}
		fmt.Fprintf(&e.out, "\"sec.ref.compare\"(%s, %s) <{predicate = %s}> ", valueName(operation.Operands[0]), valueName(operation.Operands[1]), mlirString(operation.Operator))
		return e.finishTypedOperation(operation, values)
	case semantic.OpReferenceEndBorrow:
		if operation.Reference == nil {
			return fmt.Errorf("%s requires reference metadata", operation.Kind)
		}
		fmt.Fprintf(&e.out, "\"sec.ref.end_borrow\"(%s) <{borrow_id = %d : i32, lifetime_id = %d : i32}> ", valueName(operation.Operands[0]), operation.Reference.Borrow, operation.Reference.Lifetime)
		return e.finishTypedOperation(operation, values)
	case semantic.OpReferenceGenerationFailure:
		fmt.Fprintf(&e.out, "\"sec.fail.reference_generation\"() <{reason = %s}> : () -> ()", mlirString(invalidReferenceGenerationReason))
		e.emitLocation(operation.Location)
		e.out.WriteByte('\n')
		return nil
	}
	return &UnsupportedLoweringError{Feature: string(operation.Kind), Function: function.ID}
}

// finishTypedOperation prints the generic operand -> result signature and the
// location of a schema-11 operation.
func (e *emitter) finishTypedOperation(operation semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	e.out.WriteString(": (")
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
