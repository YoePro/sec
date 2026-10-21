package sema

// compilerKnownRawPointerMembers is the single registry for raw-pointer
// operation identity, result type, signature, safety, effects and documentation.
// Receiver-inapplicable operations are not offered as usable members: void
// pointers have no typed access/arithmetic and AddBytes requires a byte pointer.
// Sema's operation-specific validators still diagnose explicit invalid calls.
// Rules: rules/compiler/compiler_known_members.md — "RawPtr[T] compiler-known methods";
// rules/memory/raw_pointers.md — §§3(4),11–14,35(1–3),(6);
// rules/platform/volatile.md — §§9–11; rules/memory/allocation.md — §20(5).
func compilerKnownRawPointerMembers(typ Type) []CompilerKnownMember {
	element := compilerKnownRawPointerElement(typ)
	if element.Kind == InvalidType || element.Kind == VoidType {
		return nil
	}
	members := []CompilerKnownMember{
		{ID: "CKM-RAWPTR-READ", Name: "Read", Result: element,
			Signature: "unsafe fn Read() T", Documentation: "Reads the pointee without acquiring pointee ownership. Ordinary access is distinct from volatile access.", Receiver: "RawPtr[T]", Rule: ruleRawPointers + " — §11"},
		{ID: "CKM-RAWPTR-WRITE", Name: "Write", Result: builtinType("void"),
			Signature: "unsafe fn Write(value: T) void", Documentation: "Writes the pointee without acquiring storage ownership or implicitly destroying a live non-trivial value. Ordinary access is distinct from volatile access.", Receiver: "RawPtr[T]", Rule: ruleRawPointers + " — §12"},
		{ID: "CKM-RAWPTR-VOLATILE-READ", Name: "VolatileRead", Result: element,
			Signature: "unsafe fn VolatileRead() T", Documentation: "Observable physical-storage read that the optimizer may not remove, merge, or reorder.", Receiver: "RawPtr[T] to a validated physical region", Rule: "rules/platform/volatile.md — §§9–11",
			TargetRestriction: "requires a target region with validated volatile access", AllocationBehavior: AllocationFree, Effects: []EffectKind{EffectVolatileRead}},
		{ID: "CKM-RAWPTR-VOLATILE-WRITE", Name: "VolatileWrite", Result: builtinType("void"),
			Signature: "unsafe fn VolatileWrite(value: T) void", Documentation: "Observable physical-storage write that the optimizer may not remove, merge, or reorder.", Receiver: "RawPtr[T] to a validated physical region", Rule: "rules/platform/volatile.md — §§9–11",
			TargetRestriction: "requires a target region with validated volatile access", AllocationBehavior: AllocationFree, Effects: []EffectKind{EffectVolatileWrite}},
		{ID: "CKM-RAWPTR-OFFSET", Name: "Offset", Result: typ,
			Signature: "unsafe fn Offset(count: int) RawPtr[T]", Documentation: "Pointer moved by count elements. The caller must ensure valid address arithmetic; this operation does not establish a safe reference.", Receiver: "typed RawPtr[T]", Rule: ruleRawPointers + " — §14"},
		{ID: "CKM-RAWPTR-DIFFERENCE", Name: "Difference", Result: builtinType("int"),
			Signature: "unsafe fn Difference(other: RawPtr[T]) int", Documentation: "Distance in elements between compatible pointers into the same allocation/address domain.", Receiver: "typed RawPtr[T]", Rule: ruleRawPointers + " — §14"},
		{ID: "CKM-RAWPTR-AS-REF", Name: "AsRef", Result: Type{Name: referenceTypeName(element, false), Kind: ReferenceType, Element: &element},
			Signature: "unsafe fn AsRef() ref T", Documentation: "Constructs a shared safe reference from this raw pointer. The reference does not own or extend the pointed-to storage and follows ordinary Sec reference rules.", Receiver: "typed RawPtr[T]", Rule: ruleRawPointers + " — §18A (MD-070)"},
		{ID: "CKM-RAWPTR-AS-REF-MUT", Name: "AsRefMut", Result: Type{Name: referenceTypeName(element, true), Kind: ReferenceType, Element: &element, ReferenceMutable: true},
			Signature: "unsafe fn AsRefMut() ref mut T", Documentation: "Constructs an exclusive mutable safe reference from this raw pointer. The reference does not own or extend the storage and follows ordinary Sec reference rules.", Receiver: "typed RawPtr[T]", Rule: ruleRawPointers + " — §18A (MD-070)"},
	}
	if isRawBytePointer(typ) {
		members = append(members, CompilerKnownMember{
			ID: "CKM-RAWPTR-ADDBYTES", Name: "AddBytes", Result: typ,
			Signature: "unsafe fn AddBytes(count: int) RawPtr[byte]", Documentation: "Pointer moved by count bytes. The caller must ensure valid address arithmetic; this operation does not establish a safe reference.", Receiver: "RawPtr[byte]", Rule: ruleRawPointers + " — §14",
		})
	}
	for index := range members {
		members[index].Kind = CompilerKnownMethod
		members[index].Unsafe = true
		members[index] = rawPointerCallerContract(members[index], typ, element)
		if members[index].TargetRestriction == "" {
			members[index].TargetRestriction = "requires target-supported raw-pointer representation and applicable access/address arithmetic contracts"
		}
	}
	return members
}

// rawPointerCallerContract supplies structured parameter and caller-obligation
// metadata for the canonical operations. Unsafe authorizes the operation class;
// these are requirements on the caller, not facts proven about a source pointer.
// Rules: rules/memory/raw_pointers.md — §§5.2(1–3),11(4–5),12(2–4),13(4–8),14–15,35;
// rules/platform/volatile.md — §§9–12.
func rawPointerCallerContract(member CompilerKnownMember, pointer, element Type) CompilerKnownMember {
	switch member.ID {
	case "CKM-RAWPTR-READ", "CKM-RAWPTR-VOLATILE-READ":
		member.CallerObligations = []string{
			"Ensure the pointer is non-null and aligned for `T`.",
			"Ensure storage for at least one `T` is readable, initialized with a valid representation, and live for the entire read.",
			"Ensure the read is allowed by ownership, borrowing, address-space and target/platform access contracts; `T` must be safely copy-readable.",
		}
	case "CKM-RAWPTR-WRITE", "CKM-RAWPTR-VOLATILE-WRITE":
		member.Parameters = []FunctionParameter{{Name: "value", Type: element}}
		member.CallerObligations = []string{
			"Ensure the pointer is non-null and aligned for `T`, and storage for at least one `T` is writable and live for the entire write.",
			"Ensure the value has a valid destination representation and the write has exclusive/mutable authority without conflicting safe references or foreign accesses.",
			"Do not overwrite a live non-trivial value without an API that explicitly proves replacement and required destruction; obey address-space and target/platform access contracts.",
		}
	case "CKM-RAWPTR-OFFSET", "CKM-RAWPTR-ADDBYTES":
		member.Parameters = []FunctionParameter{{Name: "count", Type: builtinType("int")}}
		member.CallerObligations = []string{
			"Ensure address arithmetic is supported by the selected target/address domain and does not overflow or wrap into an invalid address.",
			"Keep the resulting address in a valid allocation, mapped region or hardware window; arithmetic does not establish safe-reference validity.",
			"Before later typed access, establish alignment, lifetime, bounds, provenance, alias and target-address obligations.",
		}
	case "CKM-RAWPTR-DIFFERENCE":
		member.Parameters = []FunctionParameter{{Name: "other", Type: pointer}}
		member.CallerObligations = []string{
			"Ensure both compatible `RawPtr[T]` values refer to the same allocation/address domain with valid element positions.",
			"Ensure the selected target/address domain supports element differences; obey its pointer-width and range rules without invalid address overflow or wraparound.",
		}
	case "CKM-RAWPTR-AS-REF", "CKM-RAWPTR-AS-REF-MUT":
		member.CallerObligations = []string{
			"Ensure the pointer is non-null, aligned for `T` and addresses an initialized valid `T` in live readable storage with correct provenance and a compatible address space.",
			"Keep that storage live and free of conflicting mutation for the reference's whole live range; the reference neither owns, pins nor extends the storage.",
		}
		if member.ID == "CKM-RAWPTR-AS-REF-MUT" {
			member.CallerObligations = append(member.CallerObligations,
				"Ensure the storage is writable and that no other live safe reference or unmodeled foreign access overlaps it for the whole live range of the exclusive reference.")
		}
	}
	if len(member.Effects) > 0 {
		member.CallerObligations = append(member.CallerObligations,
			"Validate the physical region's access width, alignment, address space and platform permissions for this volatile operation.",
			"Volatile access does not establish synchronization, atomicity, hardware privilege, mapping authority, DMA ownership or cache coherence.")
	}
	return member
}
