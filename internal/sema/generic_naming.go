package sema

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateGenericTypeParameterNames enforces uppercase type names after the
// visibility prefix. Parameters stay available for recovery; appendError
// deduplicates diagnostics when a declaration participates in multiple passes.
// Rules: rules/foundations/names_scopes_visibility.md — §12 Visibility prefixes,
// §13 Type naming, §15 Generic parameters;
// rules/declarations/generics.md — §2 Generic parameter syntax.
func (a *Analyzer) validateGenericTypeParameterNames(parameters []*ast.GenericParameter) {
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		name := parameter.Name.Value
		first, _ := utf8.DecodeRuneInString(strings.TrimLeft(name, "_"))
		if unicode.IsUpper(first) {
			continue
		}
		a.addErrorAtTokenWithMetadata(parameter.Name.Token, diagnostics.InvalidGenericParameterName,
			"use an uppercase initial after any visibility prefix and update references to this type parameter",
			"generic type parameter %s must begin with an uppercase letter after any visibility prefix", name)
	}
}

// validateGenericParameterTypeShadowing rejects a generic parameter that hides
// a visible declaration: a type, enum, interface, function, or variable of the
// declaring module, or an always-available core declaration. The check runs
// when entering the generic scope, after the complete module surface has been
// registered. Unit symbols live in their own namespace.
//
// Rules: rules/foundations/names_scopes_visibility.md — §8 Shadowing,
// §15 Generic parameters (visible type and value prohibition), §17 Name
// lookup order.
func (a *Analyzer) validateGenericParameterTypeShadowing(parameters []*ast.GenericParameter) {
	module := a.currentModule
	if a.currentImplTarget != "" {
		// Nested impl members are analyzed with their owner set, but without
		// necessarily setting currentModule on this pass.
		if target, exists := a.types[a.currentImplTarget]; exists && target.Module != "" {
			module = target.Module
		}
	}
	previousModule := a.currentModule
	a.currentModule = module
	defer func() { a.currentModule = previousModule }()
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		name := parameter.Name.Value
		declaration, ok := a.visibleShadowedDeclaration(name, true)
		if !ok {
			continue
		}
		if shadowedDeclarationIsType(declaration) {
			a.reportShadowing(parameter.Name.Token, declaration,
				diagnostics.GenericParameterShadowsType,
				"generic parameter %s shadows visible %s %s", name, declaration.Kind, name)
			continue
		}
		a.reportShadowing(parameter.Name.Token, declaration,
			diagnostics.GenericParameterShadowsDeclaration,
			"generic parameter %s shadows visible %s %s", name, declaration.Kind, name)
	}
}
