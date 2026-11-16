package sema

import "strconv"

// methodGenericOverloadSet requires input-only ranking when a competing method
// template exists. Result context may complete only the uniquely selected body.
// Rules: rules/declarations/functions.md — §§19–20;
// rules/declarations/generics.md — §§9,18,20,29.
func methodGenericOverloadSet(functions []Function) bool {
	if len(functions) < 2 {
		return false
	}
	for _, function := range functions {
		if function.ImplTarget != "" && len(function.GenericParameters) > 0 {
			return true
		}
	}
	return false
}

// genericOverloadParameterTypes compares method parameters by generic position,
// not the spelling of locally bound generic names. Renaming U to V cannot
// create an overload distinguished only by its result. Owner parameters remain
// untouched, and this view never changes the registered template.
// Rules: rules/declarations/functions.md — §19;
// rules/declarations/generics.md — §§9,29–30.
func genericOverloadParameterTypes(function Function) []Type {
	parameters := make([]Type, len(function.Parameters))
	bindings := map[string]Type{}
	if function.ImplTarget != "" && function.GenericTypePack == "" {
		for index, name := range function.GenericParameters {
			bindings[name] = Type{Name: "$method" + strconv.Itoa(index), Kind: GenericType}
		}
	}
	for index, parameter := range function.Parameters {
		parameters[index] = substituteGenericType(parameter.Type, bindings)
	}
	return parameters
}
