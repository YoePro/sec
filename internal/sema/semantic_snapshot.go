package sema

// semanticSnapshotType retains structural type capabilities for post-Sema
// consumers while recursively detaching mutable slices and pointers from
// Analyzer-owned state. Named recursive edges widen to their scalar surface.
//
// Rules:
//   - rules/compiler/compiler_analysis.md — immutable analysis results
//   - rules/analysis/closure_analysis.md — "CapturedType"
//   - rules/analysis/parameter_usage_analysis.md — "Function summaries"
func semanticSnapshotType(typ Type) Type {
	return semanticSnapshotTypeSeen(typ, map[string]bool{})
}

func semanticSnapshotTypeSeen(typ Type, seen map[string]bool) Type {
	result := escapeSnapshotType(typ)
	// Associated-value enums retain their source category and closed variant
	// names in post-Sema facts, rather than becoming an anonymous union.
	// Rules: rules/concurrency/atomics.md — §§6,17,46(3–4).
	result.AssociatedEnum = typ.AssociatedEnum
	result.ErrorAssignable = typ.ErrorAssignable
	result.EnumValues = append([]string(nil), typ.EnumValues...)
	// Case values and bit-backed width let Semantic IR define an enum first
	// reached through a generic argument (monomorphization.md § 9.1). Other
	// types' BitWidth depends on the selected target and stays out of the
	// target-independent snapshot.
	if typ.Kind == EnumType {
		result.BitWidth = typ.BitWidth
	}
	if typ.EnumConsts != nil {
		result.EnumConsts = make(map[string]EnumValue, len(typ.EnumConsts))
		for name, constant := range typ.EnumConsts {
			result.EnumConsts[name] = constant
		}
	}
	result.EnumDefault = typ.EnumDefault
	result.UnionDefault = typ.UnionDefault
	result.DeclarationToken = typ.DeclarationToken
	// Bounded collection arguments are part of structural type identity, not
	// declaration bodies. Preserve them for map component and source facts.
	// Rules: rules/collections/collections.md — §§13.1,14.1,15.1.
	result.ConstArgs = append([]int64(nil), typ.ConstArgs...)
	result.ReferenceOriginAlternatives = append([]ReferenceOriginAlternative(nil), typ.ReferenceOriginAlternatives...)
	// rules/memory/destruction.md §§3.3(2), 15: post-Sema policy must
	// retain an explicit lifecycle contract even at a recursive type edge.
	result.CustomFree = typ.CustomFree
	key := ""
	if typ.Named && typ.Name != "" {
		key = typ.Module + "|" + typ.Name
		if seen[key] {
			return result
		}
		seen[key] = true
		defer delete(seen, key)
	}
	if typ.Element != nil {
		element := semanticSnapshotTypeSeen(*typ.Element, seen)
		result.Element = &element
	}
	result.TypeArgs = make([]Type, len(typ.TypeArgs))
	for index := range typ.TypeArgs {
		result.TypeArgs[index] = semanticSnapshotTypeSeen(typ.TypeArgs[index], seen)
	}
	result.Fields = make([]StructField, len(typ.Fields))
	for index, field := range typ.Fields {
		result.Fields[index] = field
		result.Fields[index].Type = semanticSnapshotTypeSeen(field.Type, seen)
		result.Fields[index].Tags = append([]StructTag(nil), field.Tags...)
	}
	// A register's bit layout is part of its post-Sema identity: receiver and
	// place facts must let Semantic IR build the same register definition as
	// a direct use of the type (MD-077 § 10(1)).
	result.RegisterAllocationOrder = typ.RegisterAllocationOrder
	result.RegisterByteOrder = typ.RegisterByteOrder
	result.RegisterShadow = typ.RegisterShadow
	// Field carriers keep their complete declared types (bit-backed enum
	// representation, nested register layout); the declaration is immutable.
	result.RegisterFields = append([]RegisterField(nil), typ.RegisterFields...)
	result.UnionVariants = make([]UnionVariant, len(typ.UnionVariants))
	for index, variant := range typ.UnionVariants {
		result.UnionVariants[index] = variant
		if variant.Payload != nil {
			payload := semanticSnapshotTypeSeen(*variant.Payload, seen)
			result.UnionVariants[index].Payload = &payload
		}
		result.UnionVariants[index].PayloadFields = make([]StructField, len(variant.PayloadFields))
		for fieldIndex, field := range variant.PayloadFields {
			result.UnionVariants[index].PayloadFields[fieldIndex] = field
			result.UnionVariants[index].PayloadFields[fieldIndex].Type = semanticSnapshotTypeSeen(field.Type, seen)
			result.UnionVariants[index].PayloadFields[fieldIndex].Tags = append([]StructTag(nil), field.Tags...)
		}
	}
	return result
}
