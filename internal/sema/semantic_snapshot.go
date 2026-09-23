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
