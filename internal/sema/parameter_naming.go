package sema

import (
	"unicode"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// coreModuleName is the module of always-available trusted core declarations.
const coreModuleName = "core"

// visibleShadowedDeclaration returns the declaration that an unqualified
// name would hide in the current module: a top-level declaration of the
// current module, or an accessible always-available core declaration.
// Declarations of imported modules are reachable only qualified and are not
// shadowed, and unit symbols live in their own namespace. Module variables
// are included only when includeVariables is set, because a local that
// repeats a module variable is already a duplicate declaration (S1004).
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §8 "Shadowing", §17 "Name lookup order"
//   - rules/types/units.md — separate unit-symbol namespace
func (a *Analyzer) visibleShadowedDeclaration(name string, includeVariables bool) (moduleDeclaration, bool) {
	accepts := func(declaration moduleDeclaration) bool {
		return declaration.Kind != moduleDeclarationUnit && validDefinitionToken(declaration.Token) &&
			(includeVariables || declaration.Kind != moduleDeclarationVariable)
	}
	if declaration, ok := a.moduleSurfaces[a.currentModule][name]; ok && accepts(declaration) {
		return declaration, true
	}
	if a.currentModule != coreModuleName && canAccessDeclaredNameFromModule(name, coreModuleName, a.currentModule) {
		if declaration, ok := a.moduleSurfaces[coreModuleName][name]; ok && accepts(declaration) &&
			!(shadowedDeclarationIsType(declaration) && contextualLowercaseTypeName(name)) {
			return declaration, true
		}
	}
	// Compiler-known nominal types such as Event or Task are always visible
	// although no source declaration exists; the result has no source token.
	// Lowercase compiler-known type names (error, time, list, ...) are
	// contextual type spellings that the rulebooks also use as ordinary
	// binding names, such as `Err(error)`, so they are not shadowed.
	if !contextualLowercaseTypeName(name) {
		if typ, ok := a.types[name]; ok && typ.Intrinsic && !a.unitTypeNames[name] && !lexer.IsReservedDeclarationName(name) {
			return moduleDeclaration{Name: name, Kind: compilerKnownTypeDeclaration}, true
		}
	}
	return moduleDeclaration{}, false
}

// contextualLowercaseTypeName reports a lowercase always-available type
// spelling such as error, time, date, or duration. The rulebooks use these as
// ordinary binding names (`Err(error)`), so bindings do not shadow them.
func contextualLowercaseTypeName(name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	return !unicode.IsUpper(first)
}

// compilerKnownTypeDeclaration marks a shadowed compiler-known type, which
// has no source declaration to point at.
const compilerKnownTypeDeclaration moduleDeclarationKind = "compiler-known type"

// reportShadowing reports name at token hiding declaration, pointing at the
// hidden declaration when it has a source location.
func (a *Analyzer) reportShadowing(token lexer.Token, declaration moduleDeclaration, id string, format string, args ...any) {
	if validDefinitionToken(declaration.Token) {
		a.addErrorAtTokenWithPreviousID(token, declaration.Token, id, format, args...)
		return
	}
	a.addErrorAtTokenWithID(token, id, format, args...)
}

func shadowedDeclarationIsType(declaration moduleDeclaration) bool {
	switch declaration.Kind {
	case moduleDeclarationType, moduleDeclarationEnum, moduleDeclarationInterface, compilerKnownTypeDeclaration:
		return true
	}
	return false
}

// validateParameterTypeShadowing rejects a function or method parameter whose
// name hides a visible declaration: a generic parameter of the declaration, a
// type, enum, interface, function, or variable of the current module, or an
// always-available core declaration. Type declarations are collected before
// callable signatures, so this also catches forward declarations. Unit
// symbols live in their own namespace and are never shadowed by a parameter.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §3 module declaration surface
//   - rules/foundations/names_scopes_visibility.md — §8 Shadowing, §17 Name lookup order, §20 Diagnostics
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func (a *Analyzer) validateParameterTypeShadowing(parameter *ast.Identifier) {
	if parameter == nil {
		return
	}
	if previous, exists := a.genericTypeDefinitions[parameter.Value]; exists && validDefinitionToken(previous) {
		a.addErrorAtTokenWithPreviousID(parameter.Token, previous, diagnostics.ParameterShadowsDeclaration,
			"parameter %s shadows visible generic parameter %s", parameter.Value, parameter.Value)
		return
	}
	declaration, ok := a.visibleShadowedDeclaration(parameter.Value, true)
	if !ok {
		return
	}
	if shadowedDeclarationIsType(declaration) {
		a.reportShadowing(parameter.Token, declaration, diagnostics.ParameterShadowsType,
			"parameter %s shadows visible %s %s", parameter.Value, declaration.Kind, parameter.Value)
		return
	}
	a.reportShadowing(parameter.Token, declaration, diagnostics.ParameterShadowsDeclaration,
		"parameter %s shadows visible %s %s", parameter.Value, declaration.Kind, parameter.Value)
}

// reportLocalShadowsDeclaration rejects a local binding (variable, loop
// binding, pattern binding, or lambda parameter) that hides a visible
// generic parameter, a type, enum, interface, or function of the current
// module, or an always-available core declaration. The binding is still
// declared so later uses do not cascade. Unit symbols occupy a separate
// namespace, so a local never shadows a unit.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §2 "One declaration namespace per scope", §8 "Shadowing"
//   - rules/foundations/names_scopes_visibility.md — §20 "Diagnostics": "local declaration count shadows visible declaration count"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func (a *Analyzer) reportLocalShadowsDeclaration(name string, token lexer.Token) {
	if previous, exists := a.genericTypeDefinitions[name]; exists && validDefinitionToken(previous) {
		a.addErrorAtTokenWithPreviousID(token, previous, diagnostics.LocalShadowsDeclaration,
			"local declaration %s shadows visible generic parameter %s", name, name)
		return
	}
	declaration, ok := a.visibleShadowedDeclaration(name, false)
	if !ok {
		return
	}
	a.reportShadowing(token, declaration, diagnostics.LocalShadowsDeclaration,
		"local declaration %s shadows visible %s %s", name, declaration.Kind, name)
}

// isUnitSymbol reports a declared unit symbol. Unit symbols occupy their own
// namespace and never conflict with, or are shadowed by, ordinary identifiers.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §2 "One declaration namespace per scope"
//   - rules/types/units.md — "Unit names and compiler-known names"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func (a *Analyzer) isUnitSymbol(name string) bool {
	_, ok := a.units[name]
	return ok
}
