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
