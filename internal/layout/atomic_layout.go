package layout

import (
	"fmt"
	"math/big"
)

// ResolvedAtomicAccess is the atomic part of an Atomic[T] layout: the
// IntegerStorage representation of the carrier, the access extent (the
// atomic value's size, never wrapper padding) and the guaranteed alignment
// of the accessed storage after the target's atomic minimum. A capability
// request names exactly these facts (atomics.md § 37A(7), (9)).
// Rules: rules/memory/layout.md — § 47.6(4).
type ResolvedAtomicAccess struct {
	Representation StorageRepresentation
	SizeBytes      uint64
	AlignBytes     uint64
}

// validateAtomicAlignments accepts verified atomic storage minima only for
// IntegerStorage representations, each a positive power of two.
// Rules: rules/memory/layout.md — §§ 8(8), 47.6(2).
func validateAtomicAlignments(facts ScalarAlignments) error {
	for representation, alignment := range facts {
		if representation.Class != IntegerStorage || !validStorageRepresentation(representation) {
			return fmt.Errorf("atomic storage alignment fact for non-integer representation %s", representation)
		}
		if alignment == 0 || alignment&(alignment-1) != 0 {
			return fmt.Errorf("atomic storage alignment %d for %s is not a positive power of two", alignment, representation)
		}
	}
	return nil
}

// resolveAtomic resolves Atomic[T] storage through the one canonical
// service. The contained type resolves as its own layout (named layers and
// integer-backed enums keep their identity and validity); the atomic access
// uses the carrier's IntegerStorage representation and the target's
// verified atomic minimum, which only raises the natural alignment. Without
// that fact the layout is incomplete, never guessed from width or host; a
// carrier that is not bool, byte or a primitive integer is invalid, a raw
// pointer stays unsupported until its representation is resolved. The
// object size is the access extent aligned to the object alignment, so
// fields and array strides automatically honour the minimum (§ 47.6(2)).
// Rules: rules/memory/layout.md — §§ 8(8), 47.3(4), 47.6;
// rules/concurrency/atomics.md — §§ 7(9), 37A(7).
func (r *Resolver) resolveAtomic(base ResolvedLayout, key string, requirement LayoutRequirement) LayoutID {
	base.Provenance = "atomic storage (layout.md § 47.6)"
	if requirement.Carrier == nil {
		base.Completeness, base.Missing = LayoutInvalid, []string{"Atomic storage without a resolved contained type"}
		return r.publish(base, key)
	}
	contained := r.child(key, *requirement.Carrier)
	if id, done := r.memo[key]; done {
		return id
	}
	base.Carrier = contained.ID
	if !contained.Complete() {
		base.Completeness, base.Missing = contained.Completeness, append([]string(nil), contained.Missing...)
		return r.publish(base, key)
	}
	scalar := contained
	for scalar.Kind == LayoutNamed || scalar.Kind == LayoutEnum {
		scalar = r.layouts[scalar.Carrier-1]
	}
	switch {
	case scalar.Kind == LayoutDescriptor:
		base.Completeness, base.Missing = LayoutUnsupported, []string{"raw-pointer atomic carrier representation (MD-095)"}
		return r.publish(base, key)
	case scalar.Kind != LayoutScalar || (scalar.Scalar != ScalarBool && scalar.Scalar != ScalarByte && scalar.Scalar != ScalarSigned && scalar.Scalar != ScalarUnsigned):
		base.Completeness, base.Missing = LayoutInvalid, []string{fmt.Sprintf("%s is not an atomic-compatible carrier (atomics.md §§ 8-9)", contained.TypeIdentity)}
		return r.publish(base, key)
	}
	size := contained.SizeBytes.Uint64()
	representation := StorageRepresentation{Class: IntegerStorage, WidthBits: uint16(size * 8)}
	minimum, known := r.plan.AtomicAlignments[representation]
	if !known {
		base.Completeness = LayoutIncomplete
		base.Missing = []string{fmt.Sprintf("atomic storage alignment of %s on %s (MD-095; layout.md § 47.6)", representation, r.key.Triple)}
		return r.publish(base, key)
	}
	alignment := new(big.Int).Set(contained.AlignBytes)
	if fact := new(big.Int).SetUint64(minimum); fact.Cmp(alignment) > 0 {
		alignment = fact
	}
	objectSize, err := r.arithmetic.AlignUp(base.TypeIdentity, contained.SizeBytes, alignment)
	if err != nil {
		base.Completeness, base.Missing = LayoutInvalid, []string{err.Error()}
		return r.publish(base, key)
	}
	base.SizeBytes, base.AlignBytes, base.Completeness = objectSize, alignment, LayoutComplete
	base.Atomic = ResolvedAtomicAccess{Representation: representation, SizeBytes: size, AlignBytes: alignment.Uint64()}
	base.ValidBitPattern = contained.ValidBitPattern
	base.Provenance = "atomic storage over " + contained.TypeIdentity + " (layout.md § 47.6)"
	return r.publish(base, key)
}
