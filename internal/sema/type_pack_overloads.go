package sema

import "sec/internal/ast"

// typePackOverloadSet routes every competing signature set containing a type
// pack to ordinary argument-only overload selection. A single pack template may
// still infer scalar generic parameters from its permitted expected context.
// Rules: rules/declarations/functions.md §38A(4)(5); rules/declarations/generics.md §29.
func typePackOverloadSet(functions []Function) bool {
	if len(functions) < 2 {
		return false
	}
	for _, function := range functions {
		if function.GenericTypePack != "" {
			return true
		}
	}
	return false
}

// partialPackOverload constructs an uncached argument signature. Constraints
// independent of unresolved parameters may eliminate a candidate; dependent
// constraints wait for completion after ordinary argument-only ranking.
// Rules: rules/declarations/generics.md §§14,18,20,29;
// rules/declarations/functions.md §38A(4)(5),(9).
func (a *Analyzer) partialPackOverload(function Function, refs []*ast.TypeReference, arguments []Type) (Function, bool) {
	var substitution map[string]Type
	var ok bool
	if function.GenericTypePack != "" {
		substitution, ok = a.deduceTypePackArguments(function, refs, arguments)
	} else {
		if len(refs) > len(function.GenericParameters) || !functionAcceptsArgumentCount(function, len(arguments)) {
			return Function{}, false
		}
		substitution, ok = map[string]Type{}, true
		fixed := map[string]struct{}{}
		for i, ref := range refs {
			typ, known := a.resolveType(ref)
			if !known {
				return Function{}, false
			}
			substitution[function.GenericParameters[i]] = typ
			fixed[function.GenericParameters[i]] = struct{}{}
		}
		for i, argument := range arguments {
			if isDeferredContextualPlaceholder(argument) {
				continue
			}
			parameter, known := functionParameterForArgument(function, i)
			if !known {
				return Function{}, false
			}
			pattern := parameter.Type
			if !typePackPatternNeedsInference(pattern) {
				continue
			}
			if pattern.Kind == ReferenceType && pattern.Element != nil && argument.Kind != ReferenceType {
				pattern = *pattern.Element
			}
			if !inferGenericTypeSubstitutionWithFixed(pattern, argument, substitution, fixed) {
				return Function{}, false
			}
		}
	}
	if !ok {
		return Function{}, false
	}
	out := function
	out.GenericParameters = nil
	out.GenericSubstitution = substitution
	out.Parameters = append([]FunctionParameter(nil), function.Parameters...)
	for i := range out.Parameters {
		out.Parameters[i].Type = substituteGenericType(out.Parameters[i].Type, substitution)
	}
	out.ReturnType = substituteGenericType(function.ReturnType, substitution)
	knownConstraints := function
	knownConstraints.GenericConstraints = nil
	for _, constraint := range function.GenericConstraints {
		if _, known := substitution[constraint.Parameter]; !known {
			continue
		}
		resolved := true
		for _, name := range function.GenericParameters {
			if _, known := substitution[name]; !known && typePackPatternNeedsInference(constraint.Interface, name) {
				resolved = false
			}
		}
		if resolved {
			knownConstraints.GenericConstraints = append(knownConstraints.GenericConstraints, constraint)
		}
	}
	out.GenericConstraintFailure = a.genericFunctionConstraintFailure(knownConstraints, substitution)
	return out, true
}

// completePackOverload uses expected context only for missing substitutions of
// the already selected declaration. Argument/explicit deductions remain fixed;
// no expected return type can remove a competing candidate or overwrite them.
// Rules: rules/declarations/generics.md §§14,18,20,29;
// rules/declarations/functions.md §38A(4)(5),(9).
func (a *Analyzer) completePackOverload(function Function, partial map[string]Type, expected Type) (Function, bool) {
	substitution := map[string]Type{}
	fixed := map[string]struct{}{}
	for name, typ := range partial {
		substitution[name] = typ
		fixed[name] = struct{}{}
	}
	missing := false
	for _, name := range function.GenericParameters {
		if _, known := substitution[name]; !known {
			missing = true
		}
	}
	if missing && expected.Kind != "" && expected.Kind != InvalidType {
		if !inferGenericTypeSubstitutionWithFixed(function.ReturnType, expected, substitution, fixed) {
			return Function{}, false
		}
	}
	for _, name := range function.GenericParameters {
		if _, known := substitution[name]; !known {
			return Function{}, false
		}
	}
	return a.instantiateGenericFunction(function, substitution), true
}
