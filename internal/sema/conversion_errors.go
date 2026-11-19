package sema

// builtinConversionErrorType supplies the canonical union shape when Sema is
// used without loaded core sources, as for the other built-in failure channels.
// The ordinary core declaration remains the source of payload/type definitions.
// Rules: rules/types/types.md — "Explicit conversions";
// rules/types/contracts.md — "Public conversion and contract errors";
// rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md — §§3.1–3.3;
// rules/types/units.md — § 22.5(12) (InvalidDomain, MD-048).
func builtinConversionErrorType(contractError Type) Type {
	return Type{
		Name: "ConversionError", Module: "core", Kind: UnionType,
		Named: true, Declared: true, ErrorAssignable: true,
		UnionVariants: []UnionVariant{
			{Name: "OutOfRange"},
			{Name: "InvalidScalar"},
			{Name: "PrecisionLoss"},
			{Name: "ScaleOverflow"},
			{Name: "NonFinite"},
			{Name: "Contract", Payload: &contractError},
			{Name: "InvalidDomain"},
		},
	}
}
