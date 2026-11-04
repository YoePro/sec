package sema

import (
	"fmt"
	"sort"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// arenaGenericRequirement is a typed Arena allocation in a generic template
// whose element type still contains template parameters. Its sized, default
// and trivial-destruction requirements are checked for every concrete
// specialization instead of being silently deferred.
type arenaGenericRequirement struct {
	method  string
	element Type
	token   lexer.Token
}

// genericInstantiation is one call that selects a specialization of a
// generic template. caller is the enclosing template when the substitution
// still names that template's parameters (a forwarded instantiation).
type genericInstantiation struct {
	function     Function
	template     sourceTokenKey
	substitution map[string]Type
	call         lexer.Token
	caller       sourceTokenKey
	forwarded    bool
}

// recordArenaGenericRequirement retains a deferred typed allocation of the
// generic template currently being analyzed.
func (a *Analyzer) recordArenaGenericRequirement(method string, element Type, token lexer.Token) {
	if a.summaryPass || !validDefinitionToken(a.currentFunctionMetadata.Token) {
		return
	}
	key := sourceTokenLocation(a.currentFunctionMetadata.Token)
	for _, existing := range a.arenaGenericRequirements[key] {
		if sameSourceToken(existing.token, token) && existing.method == method {
			return
		}
	}
	a.arenaGenericRequirements[key] = append(a.arenaGenericRequirements[key], arenaGenericRequirement{method: method, element: element, token: token})
}

// recordGenericInstantiation retains the substitution of a resolved call to
// a generic template, once per call site.
func (a *Analyzer) recordGenericInstantiation(function Function, call lexer.Token) {
	if a.summaryPass || len(function.GenericSubstitution) == 0 || !validDefinitionToken(function.Token) {
		return
	}
	instantiation := genericInstantiation{
		function:     function,
		template:     sourceTokenLocation(function.Token),
		substitution: function.GenericSubstitution,
		call:         call,
	}
	for _, typ := range function.GenericSubstitution {
		if typeMentionsGenericParameter(typ) {
			instantiation.forwarded = true
		}
	}
	if instantiation.forwarded {
		if !validDefinitionToken(a.currentFunctionMetadata.Token) {
			return
		}
		instantiation.caller = sourceTokenLocation(a.currentFunctionMetadata.Token)
		if a.reportNonConvergingSelfSpecialization(instantiation) {
			return
		}
	}
	key := sourceTokenLocation(call)
	a.genericInstantiations[key] = instantiation
}

// validateArenaGenericRequirements checks the deferred typed Arena
// allocations of every template reached from a concrete instantiation,
// including through generic templates that forward their own parameters.
// Each failure is reported at the concrete call with the allocation site as
// related location.
//
// Rules:
//   - rules/memory/arena.md — §§19(1–4), 20(2), 21(2), 120(1)
//   - rules/compiler/monomorphization.md — specialization validity
//   - rules/declarations/generics.md — template checking and specialization
func (a *Analyzer) validateArenaGenericRequirements() {
	forwarded := map[sourceTokenKey][]genericInstantiation{}
	concrete := []genericInstantiation{}
	for _, instantiation := range a.genericInstantiations {
		if instantiation.forwarded {
			forwarded[instantiation.caller] = append(forwarded[instantiation.caller], instantiation)
		} else {
			concrete = append(concrete, instantiation)
		}
	}
	sort.Slice(concrete, func(i, j int) bool { return tokenBefore(concrete[i].call, concrete[j].call) })
	reported := map[string]bool{}
	var visit func(template sourceTokenKey, substitution map[string]Type, call lexer.Token, active map[sourceTokenKey]bool)
	visit = func(template sourceTokenKey, substitution map[string]Type, call lexer.Token, active map[sourceTokenKey]bool) {
		if active[template] {
			return
		}
		active[template] = true
		defer delete(active, template)
		for _, requirement := range a.arenaGenericRequirements[template] {
			element := substituteGenericType(requirement.element, substitution)
			if typeMentionsGenericParameter(element) {
				continue
			}
			id, message, help, failed := arenaAllocationElementFailure(requirement.method, element)
			if !failed {
				continue
			}
			key := fmt.Sprintf("%s:%d:%d|%s:%d:%d|%s", call.File, call.Line, call.Column, requirement.token.File, requirement.token.Line, requirement.token.Column, typeDisplayName(element))
			if reported[key] {
				continue
			}
			reported[key] = true
			before := len(a.errors)
			a.addErrorAtTokenWithMetadataAndPrevious(call, requirement.token, id, help,
				"%s in this specialization of a generic template", message)
			if len(a.errors) > before {
				a.errors[len(a.errors)-1].RelatedLabel = "generic Arena allocation"
			}
		}
		for _, inner := range forwarded[template] {
			composed := make(map[string]Type, len(inner.substitution))
			for name, typ := range inner.substitution {
				composed[name] = substituteGenericType(typ, substitution)
			}
			visit(inner.template, composed, call, active)
		}
	}
	for _, instantiation := range concrete {
		visit(instantiation.template, instantiation.substitution, instantiation.call, map[sourceTokenKey]bool{})
	}
}

// arenaAllocationElementFailure is the shared decision of
// validateArenaAllocationElement for a concrete element type.
func arenaAllocationElementFailure(method string, element Type) (string, string, string, bool) {
	display := typeDisplayName(element)
	switch {
	case !compilerKnownSizedType(element):
		return diagnostics.ArenaUnsizedAllocationType,
			fmt.Sprintf("Arena.%s requires a sized type with complete layout, got %s", method, display),
			"Allocate a concrete type with complete sized layout.", true
	case !IsDefaultable(element):
		return diagnostics.ArenaAllocationMissingDefault,
			fmt.Sprintf("Arena.%s requires a type with a valid infallible default, but %s has no default", method, display),
			"Safe Arena allocation fully initializes every value with its compiler-defined default; give the type a valid default.", true
	case !TriviallyDestructible(element):
		return diagnostics.ArenaNonTrivialDestructionType,
			fmt.Sprintf("Arena.%s cannot allocate %s because it requires destruction", method, display),
			"Safe Arena allocation never runs destructors; allocate only trivially destructible types.", true
	}
	return "", "", "", false
}

// typeMentionsGenericParameter reports whether a type still contains an
// unresolved template parameter anywhere in its structure.
// reportNonConvergingSelfSpecialization rejects a template that calls itself
// with a parameter replaced by a type properly containing that parameter, such
// as Grow[T] calling Grow[Box[T]]: every specialization then demands a new,
// strictly larger canonical key, so the sequence provably never converges
// (monomorphization.md § 18.5). The call is not recorded, so no later pass
// follows the chain.
func (a *Analyzer) reportNonConvergingSelfSpecialization(instantiation genericInstantiation) bool {
	if instantiation.caller != instantiation.template {
		return false
	}
	names := make([]string, 0, len(instantiation.substitution))
	for name := range instantiation.substitution {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		typ := instantiation.substitution[name]
		if typ.Kind == GenericType || !typeMentionsGenericName(typ, name) {
			continue
		}
		a.addErrorAtTokenWithMetadata(instantiation.call, diagnostics.NonConvergingSpecialization,
			"Each specialization of this template demands one with a larger argument, so monomorphization never reaches a finite set. Pass the growing value through a non-generic type, or restructure the recursion so the generic arguments stay the same.",
			"recursive call to %s specializes %s as %s, which contains %s; the specializations never converge",
			instantiation.function.Name, name, typeDisplayName(typ), name)
		return true
	}
	return false
}

// typeMentionsGenericName reports whether typ contains the generic parameter
// name anywhere in its structure.
func typeMentionsGenericName(typ Type, name string) bool {
	if typ.Kind == GenericType {
		return typ.Name == name
	}
	if typ.Element != nil && typeMentionsGenericName(*typ.Element, name) {
		return true
	}
	for _, argument := range typ.TypeArgs {
		if typeMentionsGenericName(argument, name) {
			return true
		}
	}
	return false
}

func typeMentionsGenericParameter(typ Type) bool {
	if typ.Kind == GenericType {
		return true
	}
	if typ.Element != nil && typeMentionsGenericParameter(*typ.Element) {
		return true
	}
	for _, argument := range typ.TypeArgs {
		if typeMentionsGenericParameter(argument) {
			return true
		}
	}
	return false
}

func tokenBefore(left, right lexer.Token) bool {
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.Column < right.Column
}
