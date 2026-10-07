package sema

// compilerKnownRawPointerMembers exposes canonical unsafe pointer operations.
// Volatile accesses have independent observable effects and allocation-free
// contracts; these contracts do not confer ownership or mapping validity.
// Rules: rules/compiler/compiler_known_members.md — RawPtr members;
// rules/platform/volatile.md — §§9-11; rules/memory/allocation.md — §20(5).
func compilerKnownRawPointerMembers(typ Type) []CompilerKnownMember {
	return []CompilerKnownMember{
		CompilerKnownMember{ID: "CKM-RAWPTR-READ", Name: "Read", Kind: CompilerKnownMethod, Result: compilerKnownRawPointerElement(typ), Unsafe: true},
		CompilerKnownMember{ID: "CKM-RAWPTR-WRITE", Name: "Write", Kind: CompilerKnownMethod, Result: builtinType("void"), Unsafe: true},
		// rules/platform/volatile.md sections 9-11: volatile access is a
		// distinct unsafe, effectful operation, not an alias for Read/Write.
		CompilerKnownMember{ID: "CKM-RAWPTR-VOLATILE-READ", AllocationBehavior: AllocationFree, Name: "VolatileRead", Kind: CompilerKnownMethod, Result: compilerKnownRawPointerElement(typ), Unsafe: true, Effects: []EffectKind{EffectVolatileRead}},
		CompilerKnownMember{ID: "CKM-RAWPTR-VOLATILE-WRITE", AllocationBehavior: AllocationFree, Name: "VolatileWrite", Kind: CompilerKnownMethod, Result: builtinType("void"), Unsafe: true, Effects: []EffectKind{EffectVolatileWrite}},
		CompilerKnownMember{ID: "CKM-RAWPTR-OFFSET", Name: "Offset", Kind: CompilerKnownMethod, Result: typ, Unsafe: true},
		CompilerKnownMember{ID: "CKM-RAWPTR-ADDBYTES", Name: "AddBytes", Kind: CompilerKnownMethod, Result: typ, Unsafe: true},
		CompilerKnownMember{ID: "CKM-RAWPTR-DIFFERENCE", Name: "Difference", Kind: CompilerKnownMethod, Result: builtinType("int"), Unsafe: true},
	}
}
