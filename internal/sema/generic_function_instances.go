package sema

// instantiateGenericFunction caches one ordered type substitution while keeping
// late-bound reference/reset summaries synchronized with the owning declaration.
// Rules: rules/compiler/monomorphization.md §§8–9;
// rules/memory/lifetime_analysis.md §28; rules/declarations/functions.md §38A(9).
func (a *Analyzer) instantiateGenericFunction(function Function, substitution map[string]Type) Function {
	key := genericFunctionInstanceKey(function, substitution)
	key.BoundArguments = genericBoundArgumentsKey(function, substitution)
	// Complete semantic instances share one key regardless of the inference
	// path's temporary parameter ordering. Symbolic signatures stay local.
	// Rules: rules/compiler/monomorphization.md — §§4.1,4.5,6.2,6.4.
	request := function
	request.GenericSubstitution = substitution
	if identity, known := a.InstantiationIdentityOf(request); known {
		key = genericInstanceKey{Declaration: identity.Key}
	}
	if existing, ok := a.genericFuncInstances[key]; ok {
		// Reference/reset summaries are solved after signatures are registered.
		// Stable type identity must not freeze an earlier round's lifetime facts.
		existing.HasReturnOrigin = function.HasReturnOrigin
		existing.ReturnOrigin = cloneLocalReferenceOrigin(function.ReturnOrigin)
		existing.ArenaParameterResets = append([]int(nil), function.ArenaParameterResets...)
		if identity, known := a.InstantiationIdentityOf(existing); known {
			existing.InstantiationIdentity = identity.Key
		}
		a.genericFuncInstances[key] = existing
		return existing
	}

	out := function
	out.InstantiationIdentity = ""
	out.GenericSubstitution = make(map[string]Type, len(substitution))
	for name, typ := range substitution {
		out.GenericSubstitution[name] = typ
	}
	out.GenericParameters = nil
	out.GenericConstraints = make([]GenericConstraint, 0, len(function.GenericConstraints))
	for _, constraint := range function.GenericConstraints {
		constraint.Interface = substituteGenericType(constraint.Interface, substitution)
		out.GenericConstraints = append(out.GenericConstraints, constraint)
	}
	out.GenericConstraintFailure = a.genericFunctionConstraintFailure(function, substitution)
	out.Parameters = make([]FunctionParameter, 0, len(function.Parameters))
	for _, param := range function.Parameters {
		param.Type = substituteGenericType(param.Type, substitution)
		out.Parameters = append(out.Parameters, param)
	}
	out.ReturnType = substituteGenericType(function.ReturnType, substitution)
	if identity, known := a.InstantiationIdentityOf(out); known {
		out.InstantiationIdentity = identity.Key
	}
	a.genericFuncInstances[key] = out
	return out
}
