package sema

// constructionCandidates binds the owning generic arguments before overload
// resolution, contextual argument typing and signature previews. It never
// replaces the declaration registry or infers owner arguments from arguments
// passed to new: the source's constructed type already fixes them.
// Rules: rules/declarations/impl.md §§15–16,21–22;
// rules/declarations/generics.md §§8–9,17,25;
// rules/compiler/monomorphization.md §§4,8–9.
func (a *Analyzer) constructionCandidates(target Type, templates []Function) []Function {
	owner, ok := a.types[target.Name]
	if !ok || len(target.TypeArgs) == 0 || len(owner.GenericParameters) != len(target.TypeArgs) {
		return templates
	}
	bindings := make(map[string]Type, len(target.TypeArgs))
	for index, name := range owner.GenericParameters {
		bindings[name] = target.TypeArgs[index]
	}
	result := make([]Function, len(templates))
	for index, function := range templates {
		if function.Initializer {
			concrete := target
			function.ConstructionType = &concrete
			function = a.instantiateGenericFunction(function, bindings)
		}
		result[index] = function
	}
	return result
}
