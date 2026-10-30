package sema

import "sec/internal/lexer"

// ResolvedTypeDeclarationLocation returns declaration provenance recorded by
// Sema without exposing its mutable symbol tables. A generic or unresolved
// type cannot borrow the declaration of a same-spelled concrete type.
// Rules: rules/tooling/lsp.md — "Navigation and references", A.15;
// rules/declarations/generics.md — §30;
// rules/foundations/names_scopes_visibility.md — resolved declaration identity.
func (a *Analyzer) ResolvedTypeDeclarationLocation(typ Type) (lexer.Token, bool) {
	if a == nil || typ.Kind == GenericType || typ.Kind == GenericTypePackType || typ.Kind == InvalidType || typ.Kind == "" {
		return lexer.Token{}, false
	}
	token, ok := a.typeDefinitionTokens[typ.Name]
	return token, ok
}

// TypeReferenceDeclarationAt returns the single nominal declaration bound to a
// source type component. It compares compiler-owned source identities, never
// user spelling or physical representation. Builtins without source, generic
// parameters and ambiguous/unresolved references yield no invented target.
// Rules: rules/tooling/lsp.md — "Navigation and references", A.15;
// rules/foundations/names_scopes_visibility.md — resolved declaration identity.
func (a *Analyzer) TypeReferenceDeclarationAt(file string, line, column int) (lexer.Token, bool) {
	if a == nil {
		return lexer.Token{}, false
	}
	definitions := a.DefinitionsAt(file, line, column)
	if len(definitions) != 1 {
		return lexer.Token{}, false
	}
	definition := definitions[0]
	for _, nominal := range a.typeDefinitionTokens {
		if sourceTokenLocation(nominal) == sourceTokenLocation(definition) {
			return definition, true
		}
	}
	return lexer.Token{}, false
}
