package sema

// genericScopeMember returns a declaration made directly visible by the
// enclosing impl: fields, properties, events, methods, nested types and
// associated storage. It uses the registered owner surface, including extension
// fragments and members later in source order. A different owner's members
// never enter this scope; unit symbols keep their separate namespace.
// Rules: rules/foundations/names_scopes_visibility.md — §§8,15,17;
// rules/declarations/impl.md — §§9,11; rules/declarations/generics.md — §9.
func (a *Analyzer) genericScopeMember(name string) (moduleDeclaration, bool) {
	if a.currentImplTarget == "" {
		return moduleDeclaration{}, false
	}
	owner, ok := a.types[a.currentImplTarget]
	if !ok {
		return moduleDeclaration{}, false
	}
	result := moduleDeclaration{Name: name, Kind: moduleDeclarationKind("member")}
	if token, found := memberDefinitionToken(owner, name); found && validDefinitionToken(token) {
		result.Token = token
		return result, true
	}
	qualified := a.currentImplTarget + "." + name
	for _, method := range a.functions[qualified] {
		if validDefinitionToken(method.Token) {
			result.Token = method.Token
			return result, true
		}
	}
	if token, found := a.typeDefinitionTokens[qualified]; found && validDefinitionToken(token) && !a.unitTypeNames[qualified] {
		result.Token = token
		return result, true
	}
	if symbol, found := a.symbols.lookup(qualified); found && validDefinitionToken(symbol.Token) {
		result.Token = symbol.Token
		return result, true
	}
	return moduleDeclaration{}, false
}
