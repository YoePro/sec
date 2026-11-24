package sema

// instantiateGenericType substitutes concrete arguments into canonical declaration
// and declared-base facts before caching a nominal instance.
// Rules: rules/declarations/generics.md — §§8,14,22,25; rules/types/types.md — Generic and parameterized types.
func (a *Analyzer) instantiateGenericType(typ Type) Type {
	key := genericTypeInstanceKey(typ)
	if existing, ok := a.genericTypeInstances[key]; ok {
		return existing
	}

	substitution := map[string]Type{}
	for i, name := range typ.GenericParameters {
		if i < len(typ.TypeArgs) {
			substitution[name] = typ.TypeArgs[i]
		}
	}

	out := typ
	if typ.EnumCarrier != nil {
		carrier := substituteGenericType(*typ.EnumCarrier, substitution)
		out.EnumCarrier = &carrier
	}
	if typ.NamedBase != nil {
		base := substituteGenericType(*typ.NamedBase, substitution)
		out.NamedBase = &base
	}
	memberTemplate := typ
	// A named template whose representation is its own type parameter, such as
	// `type Wrapped[T] T`, initially has GenericType as its carrier. Substitute
	// that carrier before preserving the declaration's nominal identity.
	//
	// Rules:
	//   - rules/declarations/generics.md — §5.3 "Generic named type"
	//   - rules/declarations/generics.md — §17 "Substitution"
	if typ.Declared && typ.Named && typ.Kind == GenericType {
		if representation, ok := substitution[typ.Underlying]; ok {
			out = genericNamedTypeWithRepresentation(typ, representation)
			memberTemplate = out
		}
	}
	out.Fields = make([]StructField, 0, len(memberTemplate.Fields))
	out.UnionVariants = make([]UnionVariant, 0, len(memberTemplate.UnionVariants))
	out.Implements = make([]Type, 0, len(typ.Implements))
	out.InterfaceMethods = make([]Function, 0, len(memberTemplate.InterfaceMethods))
	out.GenericConstraints = make([]GenericConstraint, 0, len(typ.GenericConstraints))
	for _, constraint := range typ.GenericConstraints {
		constraint.Interface = substituteGenericType(constraint.Interface, substitution)
		out.GenericConstraints = append(out.GenericConstraints, constraint)
	}
	out.GenericParameters = nil
	recursive := false
	for _, field := range memberTemplate.Fields {
		field.Type = substituteGenericType(field.Type, substitution)
		if genericStructFieldHasDirectRecursiveStorage(out, field.Type) {
			if !recursive {
				a.addErrorAtToken(field.Token, "recursive generic type %s has infinite size", typeDisplayName(out))
			}
			recursive = true
			continue
		}
		out.Fields = append(out.Fields, field)
	}
	substituteGenericMemberSurface(&out, memberTemplate, substitution)
	for _, variant := range memberTemplate.UnionVariants {
		if variant.Payload != nil {
			payload := substituteGenericType(*variant.Payload, substitution)
			// rules/concurrency/tasks.md section 12(9): TaskOutcome[void]
			// retains its four-way union shape, but Completed has no value payload.
			// Normalize the compiler-known specialization for explicit and inferred
			// TaskOutcome types through the same generic instantiation path.
			if typ.Name == "TaskOutcome" && variant.Name == "Completed" && payload.Kind == VoidType {
				variant.Payload = nil
				out.UnionVariants = append(out.UnionVariants, variant)
				continue
			}
			if genericStructFieldHasDirectRecursiveStorage(out, payload) {
				if !recursive {
					a.addErrorAtToken(variant.Token, "recursive generic type %s has infinite size", typeDisplayName(out))
				}
				recursive = true
				continue
			}
			variant.Payload = &payload
		}
		if len(variant.PayloadFields) > 0 {
			fields := make([]StructField, 0, len(variant.PayloadFields))
			for _, field := range variant.PayloadFields {
				field.Type = substituteGenericType(field.Type, substitution)
				if genericStructFieldHasDirectRecursiveStorage(out, field.Type) {
					if !recursive {
						a.addErrorAtToken(field.Token, "recursive generic type %s has infinite size", typeDisplayName(out))
					}
					recursive = true
					continue
				}
				fields = append(fields, field)
			}
			variant.PayloadFields = fields
		}
		out.UnionVariants = append(out.UnionVariants, variant)
	}
	for _, implemented := range typ.Implements {
		out.Implements = append(out.Implements, substituteGenericType(implemented, substitution))
	}
	for _, method := range memberTemplate.InterfaceMethods {
		method.ReturnType = substituteGenericType(method.ReturnType, substitution)
		method.Parameters = append([]FunctionParameter(nil), method.Parameters...)
		for index := range method.Parameters {
			method.Parameters[index].Type = substituteGenericType(method.Parameters[index].Type, substitution)
		}
		out.InterfaceMethods = append(out.InterfaceMethods, method)
	}
	a.genericTypeInstances[key] = out
	return out
}
