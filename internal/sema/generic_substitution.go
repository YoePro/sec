package sema

// substituteGenericType replaces parameters in representation, element and
// callable facts while retaining each named declaration identity. Specialized
// struct fields must carry the same substitution as their enclosing arguments.
// Rules: rules/declarations/generics.md — §17 Substitution; rules/types/types.md — Named types;
// rules/compiler/monomorphization.md — §8.2 Scope of substitution.
func substituteGenericType(typ Type, substitution map[string]Type) Type {
	if typ.NamedBase != nil {
		base := substituteGenericType(*typ.NamedBase, substitution)
		typ.NamedBase = &base
	}
	if typ.Kind == GenericType {
		if concrete, ok := substitution[typ.Name]; ok {
			return concrete
		}
		return typ
	}
	if len(typ.TypeArgs) > 0 {
		out := typ
		out.TypeArgs = make([]Type, 0, len(typ.TypeArgs))
		for _, arg := range typ.TypeArgs {
			out.TypeArgs = append(out.TypeArgs, substituteGenericType(arg, substitution))
		}
		// Fields belong to the resolved instance, not to its old template
		// arguments. Copy the slice so one specialization cannot mutate another.
		out.Fields = append([]StructField(nil), typ.Fields...)
		for index := range out.Fields {
			out.Fields[index].Type = substituteGenericType(out.Fields[index].Type, substitution)
		}
		// Variant payloads belong to the instance in the same way, so a
		// forwarded Option[T] matches with a concrete payload.
		out.UnionVariants = append([]UnionVariant(nil), typ.UnionVariants...)
		for index := range out.UnionVariants {
			variant := &out.UnionVariants[index]
			if variant.Payload != nil {
				payload := substituteGenericType(*variant.Payload, substitution)
				variant.Payload = &payload
			}
			variant.PayloadFields = append([]StructField(nil), variant.PayloadFields...)
			for field := range variant.PayloadFields {
				variant.PayloadFields[field].Type = substituteGenericType(variant.PayloadFields[field].Type, substitution)
			}
		}
		return out
	}
	if typ.Element != nil {
		out := typ
		element := substituteGenericType(*typ.Element, substitution)
		out.Element = &element
		return out
	}
	if typ.Kind == FunctionType {
		out := typ
		out.FunctionParameterTypes = make([]Type, 0, len(typ.FunctionParameterTypes))
		for _, param := range typ.FunctionParameterTypes {
			out.FunctionParameterTypes = append(out.FunctionParameterTypes, substituteGenericType(param, substitution))
		}
		if typ.FunctionReturnType != nil {
			returnType := substituteGenericType(*typ.FunctionReturnType, substitution)
			out.FunctionReturnType = &returnType
		}
		return out
	}
	return typ
}

// SubstituteGenericType applies one canonical simultaneous substitution to a
// resolved type, for consumers that specialize a typed generic template.
// Rules: rules/compiler/monomorphization.md — §§ 8.1–8.3.
func SubstituteGenericType(typ Type, substitution map[string]Type) Type {
	return substituteGenericType(typ, substitution)
}

// GenericTypeParameters returns the ordered generic parameters of a generic
// type declaration, or nil. A method template's receiver type carries only the
// declaration name, so a specializing consumer needs this order to build the
// instance its substitution selects (`Box` with T=int is `Box[int]`).
// Rules: rules/compiler/monomorphization.md — §§ 8.2, 9.3.
func (a *Analyzer) GenericTypeParameters(name string) []string {
	if a == nil {
		return nil
	}
	if typ, ok := a.types[name]; ok {
		return append([]string(nil), typ.GenericParameters...)
	}
	return nil
}
