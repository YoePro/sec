package sema

// parameterTypeHasCustomCleanup identifies explicit lifecycle ownership in the
// value or its owned subvalues. Borrowed referents are not owned subvalues;
// noCopy alone is not a destruction contract. The check is independent of the
// body's inferred demand and of estimated copy cost.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Explicit contracts and programmer intent", "Candidate blockers"
//   - rules/memory/destruction.md — §§3.3(1–3), 3.4(2), 4(1–5), 15 "Custom free"
//   - rules/memory/storage.md — §2(1) "Value ownership is not backing-storage ownership"
//   - rules/collections/collections.md — owning list, map, and set element storage
func parameterTypeHasCustomCleanup(typ Type, visiting map[string]bool) bool {
	if typ.Kind == ReferenceType || typ.Kind == SliceType || typ.Kind == RawPtrType || typ.Kind == FunctionType {
		return false
	}
	if typ.CustomFree {
		return true
	}
	key := typeDestructionKey(typ)
	if key != "" {
		if visiting[key] {
			return false
		}
		visiting[key] = true
		defer delete(visiting, key)
	}
	switch typ.Kind {
	case ArrayType:
		return typ.Element != nil && parameterTypeHasCustomCleanup(*typ.Element, visiting)
	case StructType:
		if compilerKnownCollectionName(typ.Name) {
			for _, arg := range typ.TypeArgs {
				if parameterTypeHasCustomCleanup(arg, visiting) {
					return true
				}
			}
		}
		for _, field := range typ.Fields {
			if parameterTypeHasCustomCleanup(field.Type, visiting) {
				return true
			}
		}
	case UnionType:
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil && parameterTypeHasCustomCleanup(*variant.Payload, visiting) {
				return true
			}
			for _, field := range variant.PayloadFields {
				if parameterTypeHasCustomCleanup(field.Type, visiting) {
					return true
				}
			}
		}
	case ResultType:
		for _, arg := range typ.TypeArgs {
			if parameterTypeHasCustomCleanup(arg, visiting) {
				return true
			}
		}
	}
	return false
}
