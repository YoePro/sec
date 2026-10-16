package sema

// canCompareEquality validates scalar and aggregate equality without treating
// owning arrays as fixed-array values or erasing nominal operand identity.
// Rules: rules/collections/collections.md — §§6.1,10;
// rules/types/types.md — "Named types", "Type identity".
func canCompareEquality(left Type, right Type) bool {
	if left.Kind == InvalidType || right.Kind == InvalidType {
		return true
	}
	if !EqualityComparable(left) || !EqualityComparable(right) {
		return false
	}
	if isNumericType(left) && isNumericType(right) {
		if left.Named || right.Named || !left.Dimension.IsZero() || !right.Dimension.IsZero() {
			return sameConcreteType(left, right)
		}
		return true
	}
	if left.Kind == ArrayType || right.Kind == ArrayType {
		return sameComparableArrayType(left, right)
	}
	if left.Kind != right.Kind {
		return false
	}
	return sameConcreteType(left, right)
}

// sameComparableArrayType requires the same complete fixed-array type, including
// nominal wrappers and nested element identities. Shape agreement alone is not
// type identity, and owning T[] never gains ordinary array equality here.
// Rules: rules/collections/collections.md — §10;
// rules/types/types.md — "Named types", "Type identity".
func sameComparableArrayType(left Type, right Type) bool {
	if _, fixed := exactFixedArrayLength(left); !fixed {
		return false
	}
	if _, fixed := exactFixedArrayLength(right); !fixed {
		return false
	}
	return sameConcreteType(left, right)
}
