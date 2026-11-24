package sema

// AtomicValueIdentity keeps the complete semantic identity and the canonical
// capability carrier separate. An empty CarrierTypeID is unresolved, never a
// fabricated primitive. Symbolic requirements need concrete substitution first.
// No target layout, support or conversion is established by this record.
// Rules: rules/concurrency/atomics.md §37A(4–7); rules/types/types.md — Type identity.
type AtomicValueIdentity struct {
	SemanticTypeID string
	CarrierTypeID  string
	Symbolic       bool
}

// AtomicValueTypeIdentity is the type-identity service's atomic carrier
// projection. All names come from canonicalTypeIdentity, never diagnostic
// spellings or host types. Raw pointers preserve their complete source type
// while sharing only the currently represented canonical pointer carrier.
// Rules: rules/concurrency/atomics.md §37A(4–6); types/types.md — Named types.
func AtomicValueTypeIdentity(typ Type) AtomicValueIdentity {
	identity := AtomicValueIdentity{SemanticTypeID: canonicalTypeIdentity(typ)}
	identity.Symbolic = atomicTypeIdentitySymbolic(typ, map[*Type]bool{})
	if identity.Symbolic {
		return identity
	}
	seen := map[*Type]bool{}
	for {
		var next *Type
		if typ.NamedBase != nil {
			next = typ.NamedBase
		} else if typ.Kind == EnumType {
			next = typ.EnumCarrier
			builtin := builtinType(typ.Name)
			if next == nil && (typ.Intrinsic || (builtin.Kind == EnumType && canonicalTypeIdentity(typ) == canonicalTypeIdentity(builtin))) {
				// Compiler-owned fallback enums predate source carrier retention. Their
				// underlying spelling is the canonical builtin binding, not module lookup.
				base := builtinType(typ.Underlying)
				if base.Kind == IntType || base.Kind == UintType {
					next = &base
				}
			}
		}
		if next == nil {
			break
		}
		if seen[next] {
			return identity
		}
		seen[next] = true
		typ = *next
	}
	switch typ.Kind {
	case BoolType, IntType, UintType:
		if typ.Named {
			return identity
		} // A missing declared base is not a primitive.
	case RawPtrType:
		if typ.Named || len(typ.TypeArgs) != 1 {
			return identity
		}
		// Pointee type is retained in SemanticTypeID, not enumerated by physical
		// capability producers. Raw-pointer kind never becomes an integer kind.
		typ.TypeArgs = nil
		typ.Element = nil
	default:
		return identity
	}
	identity.CarrierTypeID = canonicalTypeIdentity(typ)
	return identity
}

// atomicTypeIdentitySymbolic finds unresolved parameters in representation
// layers and type arguments without inspecting or evaluating source syntax.
// Rules: rules/concurrency/atomics.md §37A(4); rules/declarations/generics.md §17.
func atomicTypeIdentitySymbolic(typ Type, seen map[*Type]bool) bool {
	if typ.Kind == GenericType || typ.Kind == GenericTypePackType || len(typ.GenericParameters) > len(typ.TypeArgs) {
		return true
	}
	for _, argument := range typ.TypeArgs {
		if atomicTypeIdentitySymbolic(argument, seen) {
			return true
		}
	}
	for _, child := range []*Type{typ.NamedBase, typ.EnumCarrier, typ.Element} {
		if child != nil && !seen[child] {
			seen[child] = true
			if atomicTypeIdentitySymbolic(*child, seen) {
				return true
			}
		}
	}
	return false
}
