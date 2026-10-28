package sema

// CopyClassificationOf derives copying permission from the concrete resolved
// type, including nominal restrictions and recursively owned fields/payloads.
// Rules: rules/memory/copy_move.md — §§3–4.
func CopyClassificationOf(typ Type) CopyClassification {
	return copyClassificationOf(typ, map[string]bool{})
}

// TriviallyCopyable reports an ordinary bit-preserving copy with no ownership transfer.
// Rules: rules/memory/copy_move.md — §§3.1,4.
func TriviallyCopyable(typ Type) bool {
	return CopyClassificationOf(typ) == CopyTrivial
}

// MoveOnly identifies owners that require transfer rather than implicit copying.
// Rules: rules/memory/copy_move.md — §§3.3,3.5,4.
func MoveOnly(typ Type) bool {
	return CopyClassificationOf(typ) == CopyMoveOnly
}

// requiresOwnershipTransfer keeps non-copyable and unproven generic values out
// of implicit copy contexts. Rules: rules/memory/copy_move.md — §§3–5.
func requiresOwnershipTransfer(typ Type) bool {
	switch CopyClassificationOf(typ) {
	case CopyMoveOnly, CopyNonCopyable, CopyConditional:
		return true
	default:
		return false
	}
}

// copyClassificationOf derives traits with a path-local concrete-instantiation
// guard; genuine recursive/unresolved ownership remains conditional.
// Rules: rules/memory/copy_move.md — §§3–4; rules/declarations/generics.md — §16 "Ownership, copying and borrowing".
func copyClassificationOf(typ Type, visiting map[string]bool) CopyClassification {
	// Canonical @noCopy guards and standalone Contexts are movable owners,
	// never implicitly copyable values.
	// Rules: rules/concurrency/mutex.md — §§6(2–4),8(1–4),28.
	if typ.Intrinsic && typ.Kind == StructType && (typ.Name == "MutexGuard" || typ.Name == "Context") {
		return CopyMoveOnly
	}
	if typ.ExplicitlyNonCopyable {
		return CopyNonCopyable
	}
	// MD-025.1 (copy_move.md § 3): a nominal type whose implementation
	// defines custom free is MoveOnly regardless of its fields; an implicit
	// copy would run free twice for one resource. Containing aggregates
	// inherit this through the ordinary derivation below.
	if typ.CustomFree {
		return CopyMoveOnly
	}
	switch typ.Kind {
	case InvalidType, VoidType, NeverType:
		return CopyNonCopyable
	case HardwareInstanceType:
		// MD-077 § 3(5): copying or moving an instance never duplicates
		// hardware authority; the instance is borrowed, not copied.
		return CopyNonCopyable
	case AnyType:
		return CopyConditional
	case BoolType,
		IntType,
		UintType,
		FloatType,
		DecimalType,
		CharType,
		RuneType,
		RawPtrType,
		FunctionType,
		InterfaceType,
		RegisterType,
		EnumType,
		StringType,
		SliceType,
		UnitDimensionType:
		return CopyTrivial
	case ReferenceType:
		if typ.ReferenceMutable {
			return CopyMoveOnly
		}
		return CopyTrivial
	case StructType:
		switch typ.Name {
		case "Task",
			"Thread",
			"Arena",
			"MutexGuard",
			"Subscription",
			"Channel",
			"Sender",
			"Receiver",
			"MessageTicket":
			return CopyMoveOnly
		case "Mutex",
			"Atomic",
			"ThreadLocal",
			"Event",
			"EventStorage",
			"ChannelOptions",
			"SenderID":
			return CopyNonCopyable
		}
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return CopyConditional
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		fields := make([]Type, 0, len(typ.Fields))
		for _, field := range typ.Fields {
			fields = append(fields, field.Type)
		}
		return aggregateCopyClassification(fields, visiting)
	case GenericType:
		return CopyConditional
	case ArrayType:
		// rules/collections/collections.md; correction26.md: an owning
		// dynamic array transfers its allocation regardless of element traits.
		if arrayShapeOf(typ) == ArrayShapeDynamic {
			return CopyMoveOnly
		}
		if typ.Element == nil {
			return CopyTrivial
		}
		return aggregateCopyClassification([]Type{*typ.Element}, visiting)
	case ResultType:
		return aggregateCopyClassification(typ.TypeArgs, visiting)
	case UnionType:
		key := typeDestructionKey(typ)
		if key != "" {
			if visiting[key] {
				return CopyConditional
			}
			visiting[key] = true
			defer delete(visiting, key)
		}
		parts := []Type{}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				parts = append(parts, *variant.Payload)
			}
			for _, field := range variant.PayloadFields {
				parts = append(parts, field.Type)
			}
		}
		return aggregateCopyClassification(parts, visiting)
	default:
		return CopyNonCopyable
	}
}

// aggregateCopyClassification combines owned components without manufacturing
// a copy permission for move-only, prohibited or unresolved payloads.
// Rules: rules/memory/copy_move.md — §§4.1–4.3.
func aggregateCopyClassification(parts []Type, visiting map[string]bool) CopyClassification {
	result := CopyTrivial
	for _, part := range parts {
		classification := copyClassificationOf(part, visiting)
		switch classification {
		case CopyMoveOnly:
			return CopyMoveOnly
		case CopyNonCopyable:
			return CopyNonCopyable
		case CopyConditional:
			if result == CopyTrivial {
				result = CopyConditional
			}
		case CopySemantic:
			if result == CopyTrivial {
				result = CopySemantic
			}
		}
	}
	return result
}
