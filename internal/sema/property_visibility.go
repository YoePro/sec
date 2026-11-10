package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"strings"
)

// checkPropertyVisibility enforces exact module and declaring-type owner
// boundaries before either accessor is read, written or published as resolved.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.1–12.3;
// rules/declarations/properties.md — §§2,4–5;
// rules/library/core-library.md — §1.2 (trusted core authority).
func (a *Analyzer) checkPropertyVisibility(receiver Type, property Property, use lexer.Token) bool {
	owner := a.propertyDeclaringOwner(receiver, property)
	module := property.DeclaringModule
	if module == "" {
		module = owner.Module
	}
	visible := true
	switch {
	case strings.HasPrefix(property.Name, "__"):
		current, exists := a.types[a.currentImplTarget]
		visible = exists && a.currentImplTarget != "" && current.Module == owner.Module && current.Name == owner.Name && a.currentModule == module
	case strings.HasPrefix(property.Name, "_"):
		visible = a.currentModule == module
	}
	if strings.HasPrefix(property.Name, "_") && a.isTrustedCoreSourceToken(property.Token) && !a.isTrustedCoreSourceToken(use) {
		visible = false
	}
	if visible {
		return true
	}
	before := len(a.errors)
	a.addErrorAtTokenWithMetadataAndPrevious(use, property.Token, diagnostics.PropertyNotVisible,
		"Access public properties from other modules. A single underscore is module-internal; a double underscore is private to the declaring type and its same-module implementations.",
		"property %s.%s is not visible from this context", typeDisplayName(owner), property.Name)
	if len(a.errors) > before {
		a.errors[len(a.errors)-1].RelatedLabel = "property declared here"
	}
	return false
}

// propertyDeclaringOwner follows represented nominal/core inheritance without
// treating a wrapper or another type in the module as the private owner.
// Generic substitutions retain the original property's declaration token.
// Rules: rules/foundations/names_scopes_visibility.md — §12.3;
// rules/compiler/compiler_known_members.md — Named and related types.
func (a *Analyzer) propertyDeclaringOwner(receiver Type, property Property) Type {
	owner := dereferenceType(receiver)
	seen := map[string]bool{}
	for {
		key := owner.Module + "::" + owner.Name
		if seen[key] {
			return dereferenceType(receiver)
		}
		seen[key] = true
		declared := owner
		if current, ok := a.types[owner.Name]; ok && current.Module == owner.Module {
			declared = current
		}
		for _, candidate := range declared.Properties {
			if candidate.Name == property.Name && candidate.Token == property.Token {
				return declared
			}
		}
		if owner.NamedBase == nil {
			return owner
		}
		owner = dereferenceType(*owner.NamedBase)
	}
}

// lookupPropertyOnMember resolves an instance write target without invoking a
// getter, preserving write-only properties and enforcing property visibility.
// Rules: rules/declarations/properties.md — §§4–5;
// rules/foundations/names_scopes_visibility.md — §§12.1–12.3.
func (a *Analyzer) lookupPropertyOnMember(expr *ast.MemberExpression) (Property, bool) {
	objectType, _ := a.inferPlaceBase(expr.Object)
	if objectType.Kind == InvalidType {
		return Property{}, false
	}
	property, ok := a.lookupResolvedProperty(objectType, expr.Property.Value)
	if ok && !property.Static && !a.checkPropertyVisibility(objectType, property, expr.Property.Token) {
		property.Type = Type{Kind: InvalidType}
	}
	return property, ok && !property.Static
}

// lookupStaticPropertyOnMember implements the type-qualified write boundary
// from rules/declarations/static.md, sections 8 and 11, together with
// rules/foundations/names_scopes_visibility.md §§12.1–12.3.
func (a *Analyzer) lookupStaticPropertyOnMember(expr *ast.MemberExpression) (Property, bool) {
	if root, ok := expressionRootIdentifier(expr.Object); ok && root == "self" {
		return Property{}, false
	}
	path, ok := typePathFromExpression(expr.Object)
	if !ok {
		return Property{}, false
	}
	typ, ok := a.types[a.resolveTypeName(path)]
	if !ok {
		return Property{}, false
	}
	property, ok := lookupProperty(typ, expr.Property.Value)
	if ok && property.Static && !a.checkPropertyVisibility(typ, property, expr.Property.Token) {
		property.Type = Type{Kind: InvalidType}
	}
	return property, ok && property.Static
}
