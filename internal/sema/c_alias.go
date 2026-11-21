package sema

import (
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// CBindingLayer is the semantic layer of a selected C binding module
// (MD-021 correction § 8.6(2)).
type CBindingLayer uint8

const (
	CBindingCommon CBindingLayer = iota + 1
	CBindingEnvironment
)

// String names the layer in diagnostics.
// Rules: MD-021 correction § 8.6(2).
func (layer CBindingLayer) String() string {
	if layer == CBindingEnvironment {
		return "environment"
	}
	return "common"
}

// CBindingSurface is the frozen declaration-source set of the selected C
// binding environment: every source file of a selected binding module with
// its layer. It is produced by the CompilationPlan's binding resolution and
// never by the declarations it authorizes.
//
// Rules: rules/platform/ffi.md — § 56(2), (4)-(5); MD-021 correction §§ 8.6(1)-(5).
type CBindingSurface struct {
	// Identity names the selected package@version/environment for messages.
	Identity string
	Files    map[string]CBindingLayer
}

// SetCBindingSurface snapshots the selected binding surface before
// declaration collection. Without one, no source file may declare @cAlias
// and every c::namespace::name reports S1078.
// Rules: MD-021 correction § 8.6(5); rules/foundations/attributes.md — § 95(1).
func (a *Analyzer) SetCBindingSurface(surface CBindingSurface) {
	a.recordConfiguration(func(b *Analyzer) { b.SetCBindingSurface(surface) })
	clone := CBindingSurface{Identity: surface.Identity, Files: map[string]CBindingLayer{}}
	for file, layer := range surface.Files {
		clone.Files[filepath.Clean(file)] = layer
	}
	a.cBindingSurface = clone
}

type cAliasState uint8

const (
	cAliasUnresolved cAliasState = iota
	cAliasResolving
	cAliasResolved
	cAliasFailed
)

// cAliasDefinition is one selected alias after layer specialization.
type cAliasDefinition struct {
	declaration *ast.CAliasDeclaration
	layer       CBindingLayer
	module      string
	state       cAliasState
	typ         Type
}

// CAliasFact is the resolved, transparent target of one selected alias.
type CAliasFact struct {
	Name   string
	Layer  CBindingLayer
	Module string
	Target Type
}

// CAliasFacts lists the resolved alias surface in name order; failed
// aliases are absent. It is the alias part of the binding surface that the
// resolution fingerprint and tooling consume.
// Rules: MD-021 correction §§ 8.6(4), 8.8(1).
func (a *Analyzer) CAliasFacts() []CAliasFact {
	names := make([]string, 0, len(a.cAliases))
	for name, definition := range a.cAliases {
		if definition.state == cAliasResolved {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	facts := make([]CAliasFact, len(names))
	for index, name := range names {
		definition := a.cAliases[name]
		facts[index] = CAliasFact{Name: name, Layer: definition.layer, Module: definition.module, Target: definition.typ}
	}
	return facts
}

// collectCAliasDeclarations is the declaration-only pre-freeze phase of the
// binding surface: it authorizes each @cAlias by its file's selected binding
// layer, checks the c::namespace::name spelling, specializes one common
// definition by one environment definition, rejects same-layer duplicates
// and then resolves the whole alias graph. It runs no user code.
//
// Rules:
//   - rules/platform/ffi.md — § 56(2)-(5)
//   - rules/foundations/attributes.md — § 95(1)
//   - rules/corrections/applied/MD-021-c-binding-environment-complete-proposal-20261120-20261121.md — §§ 3(3)-(5), 8.6(2)-(5)
func (a *Analyzer) collectCAliasDeclarations(program *ast.Program) {
	a.cAliases = map[string]*cAliasDefinition{}
	layers := map[string]map[CBindingLayer]*cAliasDefinition{}
	a.withProgramModules(program, func(statement ast.Statement) {
		declaration, ok := statement.(*ast.CAliasDeclaration)
		if !ok || declaration.Invalid || declaration.Name == nil || declaration.Target == nil {
			return
		}
		layer, authorized := a.cBindingSurface.Files[filepath.Clean(declaration.Token.File)]
		if !authorized {
			a.addErrorAtTokenWithMetadata(declaration.Token, diagnostics.CAliasOutsideBindingModule,
				"@cAlias is valid only in a module of the selected C binding environment (.sec/platform/c-bindings.toml); declare an ordinary type here instead.",
				"@cAlias %q is not in a module of the selected C binding environment", declaration.Name.Value)
			return
		}
		name := declaration.Name.Value
		if problem := cAliasNameProblem(name); problem != "" {
			a.addErrorAtTokenWithMetadata(declaration.Name.Token, diagnostics.CAliasInvalidName,
				"Write a qualified binding name such as \"c::stddef::size_t\"; one-segment c:: names are the compiler-owned fundamental C types.",
				"@cAlias name %q %s", name, problem)
			return
		}
		if layers[name] == nil {
			layers[name] = map[CBindingLayer]*cAliasDefinition{}
		}
		if previous := layers[name][layer]; previous != nil {
			first := previous.declaration.Name.Token
			a.addErrorAtTokenWithMetadata(declaration.Name.Token, diagnostics.CAliasDuplicate,
				"Keep one definition per layer; only an environment module may specialize a common definition.",
				"@cAlias %q is declared twice on the %s layer; first declared at %s:%d:%d", name, layer, first.File, first.Line, first.Column)
			return
		}
		layers[name][layer] = &cAliasDefinition{declaration: declaration, layer: layer, module: a.currentModule}
	})
	names := make([]string, 0, len(layers))
	for name, byLayer := range layers {
		names = append(names, name)
		if definition := byLayer[CBindingEnvironment]; definition != nil {
			a.cAliases[name] = definition
		} else {
			a.cAliases[name] = byLayer[CBindingCommon]
		}
	}
	sort.Strings(names)
	for _, name := range names {
		a.resolveCAlias(a.cAliases[name])
	}
}

// cAliasNameProblem returns why a spelling is not a binding alias name: it
// must be c:: followed by at least two identifier segments.
// Rules: rules/platform/ffi.md — §§ 5-6, 56(2); MD-021 correction § 3(3).
func cAliasNameProblem(name string) string {
	if !strings.HasPrefix(name, "c::") {
		return "must start with c::"
	}
	segments := strings.Split(strings.TrimPrefix(name, "c::"), "::")
	if len(segments) < 2 {
		return "names the compiler-owned fundamental C namespace, which no binding can redefine"
	}
	for _, segment := range segments {
		if segment == "" || !(segment[0] == '_' || segment[0] >= 'a' && segment[0] <= 'z' || segment[0] >= 'A' && segment[0] <= 'Z') {
			return "has an empty or non-identifier segment"
		}
		for _, char := range segment {
			if !(char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
				return "has an empty or non-identifier segment"
			}
		}
	}
	return ""
}

// resolveCAlias resolves one alias's TypeReference in its declaring module,
// following other aliases transparently, rejecting cycles and targets that
// cannot cross a C ABI boundary in any position.
// Rules: rules/platform/ffi.md — § 56(3); MD-021 correction §§ 3(4), 8.6(4).
func (a *Analyzer) resolveCAlias(definition *cAliasDefinition) (Type, bool) {
	switch definition.state {
	case cAliasResolved:
		return definition.typ, true
	case cAliasFailed:
		return Type{Kind: InvalidType}, false
	case cAliasResolving:
		definition.state = cAliasFailed
		a.addErrorAtTokenWithMetadata(definition.declaration.Name.Token, diagnostics.CAliasCycle,
			"Make one alias in the chain name a concrete C type instead of another alias.",
			"@cAlias %q is part of a circular alias chain", definition.declaration.Name.Value)
		return Type{Kind: InvalidType}, false
	}
	definition.state = cAliasResolving
	previousModule, previousFile := a.currentModule, a.currentSourceFile
	a.currentModule, a.currentSourceFile = definition.module, definition.declaration.Token.File
	typ, ok := a.resolveType(definition.declaration.Target)
	a.currentModule, a.currentSourceFile = previousModule, previousFile
	if definition.state == cAliasFailed {
		return Type{Kind: InvalidType}, false
	}
	if !ok || typ.Kind == InvalidType {
		definition.state = cAliasFailed
		return Type{Kind: InvalidType}, false
	}
	verdict := foreignTypeLegality(typ, foreignParameterByValue)
	for _, position := range []foreignPosition{foreignParameterReferent, foreignReturn} {
		if !verdict.legal {
			verdict = foreignTypeLegality(typ, position)
		}
	}
	if !verdict.legal || typ.Kind == GenericType {
		definition.state = cAliasFailed
		reason := verdict.reason
		if reason == "" {
			reason = "an alias target must be a concrete type"
		}
		a.addErrorAtTokenWithMetadata(definition.declaration.Target.Token, diagnostics.CAliasIllegalTarget, verdict.help,
			"@cAlias %q targets %s, which cannot cross a C ABI boundary: %s", definition.declaration.Name.Value, typeDisplayName(typ), reason)
		return Type{Kind: InvalidType}, false
	}
	definition.state, definition.typ = cAliasResolved, typ
	return typ, true
}

// resolveCAliasUse resolves a c::namespace::name use to its alias target.
// The alias is transparent: the result is the target type itself, so no
// nominal identity is introduced and ABI checks see the target.
// Rules: rules/platform/ffi.md — § 56(3); MD-021 correction § 8.6(4).
func (a *Analyzer) resolveCAliasUse(ref *ast.TypeReference) (Type, bool, bool) {
	definition, found := a.cAliases[ref.Name]
	if !found {
		return Type{}, false, false
	}
	if len(ref.TypeArgs) != 0 || len(ref.ConstArgs) != 0 {
		a.addErrorAtToken(ref.Token, "%s is a C alias and takes no type arguments", ref.Name)
		return Type{Kind: InvalidType}, false, true
	}
	a.bindDefinition(typeReferenceNominalToken(ref), definition.declaration.Name.Token)
	typ, ok := a.resolveCAlias(definition)
	return typ, ok, true
}
