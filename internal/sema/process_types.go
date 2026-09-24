package sema

// processIDType constructs the compiler-known nominal process identity using
// the selected target's canonical unsigned machine width. It represents Sec's
// stable execution identity, not an operating-system process identifier or an
// owning lifecycle capability.
//
// Rules:
//   - rules/concurrency/processes.md — §5.1 "ProcessID"
func processIDType(width uint16) Type {
	typ := targetUnsignedIntegerType("ProcessID", width)
	typ.Named = true
	typ.Underlying = "uint"
	return typ
}

// processStatusType constructs the exhaustive portable process lifecycle
// status enum. Runtime-private states must normalize to these source-visible
// variants rather than extending this compiler-known type.
//
// Rules:
//   - rules/concurrency/processes.md — §5.2 "ProcessStatus"
func processStatusType() Type {
	variants := []string{
		"Created",
		"Running",
		"Completed",
		"CompletionFailed",
		"Panicked",
		"Terminated",
	}
	return Type{
		Name:        "ProcessStatus",
		Kind:        EnumType,
		Named:       true,
		Intrinsic:   true,
		Underlying:  "uint",
		EnumValues:  variants,
		EnumConsts:  builtinEnumConsts(variants),
		EnumDefault: "Created",
	}
}
