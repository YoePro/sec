package sema

// builtinAtomicType reserves the nominal core storage family. Its physical
// representation is compiler-owned and has no ordinary public fields.
// Rules: rules/concurrency/atomics.md — §§4,7,11;
// rules/corrections/applied/atomics-memory-model-v2-cross-rulebook-correction-20260907.md — §2.
func builtinAtomicType() Type {
	return Type{Name: "Atomic", Module: "core", Kind: StructType, Named: true,
		Intrinsic: true, GenericParameters: []string{"T"}}
}

// builtinMemoryOrderType supplies the predeclared core identity for standalone
// Sema. The trusted core source declaration refines this same reserved name.
// Rules: rules/concurrency/atomics.md — §§4–5.
func builtinMemoryOrderType() Type {
	values := []string{"Relaxed", "Acquire", "Release", "AcqRel", "SeqCst"}
	return Type{Name: "MemoryOrder", Module: "core", Named: true, Declared: true,
		Kind: EnumType, Underlying: "int", EnumValues: values,
		EnumConsts: builtinEnumConsts(values), EnumDefault: "Relaxed"}
}

// atomicElementTypeSupported checks language eligibility independently of
// target width, native instructions, lock freedom or permitted emulation.
// Integer-backed enum range metadata distinguishes it from string-backed enums,
// including enums over named integer carriers and bit-width representations.
// Rules: rules/concurrency/atomics.md — §§7–10,27–30.
func atomicElementTypeSupported(typ Type) bool {
	switch typ.Kind {
	case BoolType, RawPtrType, IntType, UintType:
		return true
	case EnumType:
		return typ.MinInteger != nil || typ.MaxInteger != nil || isIntegerType(builtinType(typ.Underlying))
	default:
		return false
	}
}

// isAtomicType identifies the compiler-owned atomic family after reference
// projection; a same-spelled user struct is not atomic storage.
// Rules: rules/concurrency/atomics.md — §§4,7.
func isAtomicType(typ Type) bool {
	return isAtomicFamilyIdentity(typ) && len(typ.TypeArgs) == 1
}

// isAtomicFamilyIdentity recognizes the nominal compiler-owned core family,
// independently of whether this is its template or a closed instance.
// Rules: rules/concurrency/atomics.md — §§4,7.
func isAtomicFamilyIdentity(typ Type) bool {
	return typ.Intrinsic && typ.Named && typ.Module == "core" && typ.Name == "Atomic" && typ.Kind == StructType
}

// atomicOperationApplicability supplies the shared semantic operation matrix
// for Sema and tooling. All eligible carriers have the common surface; fetch
// operations require integer carriers whose contracts need no additional
// atomic failure/result model. Unknown names stay outside the atomic API.
// This says nothing about concrete target/lock-free/emulation capability.
// Rules: rules/concurrency/atomics.md — §§8–10,12–13,21–22,27–30,43–45.
func atomicOperationApplicability(operation string, element Type) (supported, known bool) {
	switch operation {
	case "Load", "Store", "Swap", "CompareExchange":
		return atomicElementTypeSupported(element), true
	case "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
		return atomicElementTypeSupported(element) && isIntegerType(element) && element.Kind != EnumType && len(element.Contracts) == 0, true
	default:
		return false, false
	}
}
