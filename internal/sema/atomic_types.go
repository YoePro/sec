package sema

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
	return typ.Intrinsic && typ.Name == "Atomic" && len(typ.TypeArgs) == 1
}

// atomicFetchOperationSupported restricts integer fetch operations to integer
// carriers whose contracts need no additional atomic failure/result model.
// Rules: rules/concurrency/atomics.md — §§13,27(5–6),28–30.
func atomicFetchOperationSupported(operation string, element Type) bool {
	switch operation {
	case "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
		return isIntegerType(element) && element.Kind != EnumType && len(element.Contracts) == 0
	default:
		return false
	}
}
