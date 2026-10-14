package sema

// builtinCollectionErrorType owns the canonical public structural failure
// family. Fresh case metadata belongs to each newly constructed builtin
// registration. AllocationError remains a distinct lower-level type; conversion at
// collection API boundaries is not an implicit type conversion.
// Rules: rules/collections/collections.md — §16 "Collection errors";
// rules/library/core-library.md — §20 "Standard core errors";
// rules/corrections/applied/md058-collection-capacity-reservation-correction-20261012.md — §5.
func builtinCollectionErrorType() Type {
	cases := []string{"AllocationFailed", "CapacityExceeded", "SizeOverflow"}
	return Type{
		Name: "CollectionError", Kind: EnumType, Underlying: "uint",
		EnumValues: cases, EnumConsts: builtinEnumConsts(cases), ErrorAssignable: true,
	}
}
