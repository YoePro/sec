package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// typeFromDeclaration builds a named type without erasing inherited contracts.
// Rules: rules/types/types.md — Named types; rules/types/contracts.md — Composition.
func (a *Analyzer) typeFromDeclaration(stmt *ast.TypeDeclStatement, baseType Type) Type {
	return a.typeFromDeclarationWithName(stmt.Name.Value, stmt, baseType)
}

// typeFromDeclarationWithName records nominal identity, contract conjunction,
// explicit-default validation and their source provenance.
// Rules: rules/types/types.md — Named types; rules/types/contracts.md — Composition, Diagnostics;
// rules/types/default_values.md — Explicit type defaults.
func (a *Analyzer) typeFromDeclarationWithName(name string, stmt *ast.TypeDeclStatement, baseType Type) Type {
	typ := baseType
	base := baseType
	typ.NamedBase = &base
	typ.FixedNamedArguments = len(stmt.GenericParameters) == 0 && (len(baseType.TypeArgs) > 0 || len(baseType.ConstArgs) > 0)
	if baseType.Kind == EnumType || baseType.Kind == UnionType {
		typ.ConstantAncestors = append(append([]string(nil), baseType.ConstantAncestors...), baseType.Name)
		if typ.ConstantBaseName == "" {
			typ.ConstantBaseName = baseType.Name
		}
	}
	typ.DeclarationToken = stmt.Name.Token
	typ.Name = name
	typ.Module = a.currentModule
	typ.Named = true
	typ.Declared = true
	if hasAttribute(stmt.Attributes, "noCopy") {
		typ.ExplicitlyNonCopyable = true
		typ.NoCopyPolicyOrigin = name
	} else if typ.ExplicitlyNonCopyable && typ.NoCopyPolicyOrigin == "" {
		typ.NoCopyPolicyOrigin = baseType.Name
	}
	typ.Underlying = baseType.Name
	typ.Contracts = append([]Contract(nil), baseType.Contracts...)
	typ.GenericParameters = genericParameterNameValues(stmt.GenericParameters)
	typ.GenericConstraints = a.resolvedGenericParameterConstraints(stmt.GenericParameters)

	if stmt.BaseType != nil && stmt.BaseType.Unit != "" {
		typ.Unit = stmt.BaseType.Unit
		typ.Dimension = a.parseDimension(stmt.BaseType.Unit)
	}
	if stmt.AssignedType != nil && stmt.AssignedType.Unit != "" {
		typ.Unit = stmt.AssignedType.Unit
		typ.Dimension = a.parseDimension(stmt.AssignedType.Unit)
	}

	typ = a.applyContracts(typ, stmt.Contract)
	if stmt.Default != nil {
		typ.ExplicitDefaultToken = expressionToken(stmt.Default)
		start := len(a.errors)
		defer func() { a.relateErrorsSince(start, typ.DeclarationToken, "type declaration") }()
		// MD-011: the default is an ordinary expression in a
		// SemanticCompileTimeRequiredContext.
		constant, outcome := a.semanticCompileTimeConstantVisiting(stmt.Default, map[string]bool{}, typ)
		ok := outcome == compileTimeEvaluated
		if outcome == compileTimeRequiresExecution || outcome == compileTimeForbiddenClock {
			typ.InvalidExplicitDefault = true
			a.reportCompileTimeRequirement(stmt.Default, outcome, "default", diagnostics.InvalidExplicitDefault, "")
		} else if !ok {
			typ.InvalidExplicitDefault = true
			a.addErrorAtTokenWithMetadata(stmt.DefaultToken, diagnostics.InvalidExplicitDefault, "use an allocation-free compile-time constant", "default for %s must be a compile-time primitive constant", name)
		} else if !defaultRepresentable(typ, constant) {
			// rules/types/default_values.md, "Explicit type defaults" and
			// "Diagnostics": representability is checked before contracts.
			typ.InvalidExplicitDefault = true
			a.addErrorAtTokenWithMetadata(expressionToken(stmt.Default), diagnostics.DefaultNotRepresentable, "choose a value representable by "+contractApplicabilityTypeName(typ), "default value %s is not representable by %s", stmt.Default.String(), name)
		} else if violated, ok := firstViolatedContract(typ, constant); ok {
			// rules/types/contracts.md, "Explicit defaults" and "Diagnostics":
			// name the first violated contract in source order.
			typ.InvalidExplicitDefault = true
			a.addContractError(expressionToken(stmt.Default), violated, diagnostics.DefaultViolatesContract, fmt.Sprintf("%s requires %s; choose a value satisfying every type contract", name, describeContract(violated)), "default value %s is invalid for %s", stmt.Default.String(), name)
		} else {
			typ.ExplicitDefault = &constant
		}
	}
	return typ
}

// isCoreBuiltinDeclaration identifies compiler-owned names with trusted core declarations.
// Rules: rules/library/core-library.md; rules/concurrency/mutex.md §13;
// rules/types/contracts.md "Public conversion and contract errors" (ContractError, MD-012).
func isCoreBuiltinDeclaration(name string) bool {
	switch name {
	case "IndexError", "FormatError", "StringError", "ContractError", "TaskOutcome", "TaskSpawnError", "TaskError",
		"date", "time", "datetime", "duration", "Instant":
		return true
	default:
		return false
	}
}

// isTrustedCoreBuiltinDeclaration identifies the narrow set of compiler-known
// types whose fallback identity is refined by a real declaration in loader-
// proven core source. The source declaration supplies storage/member shape; it
// never transfers ownership of the language-reserved name to ordinary source.
//
// Rules: rules/library/core-library.md; rules/types/temporal.md §2.
func (a *Analyzer) isTrustedCoreBuiltinDeclaration(name string, token lexer.Token) bool {
	return isCoreBuiltinDeclaration(name) && a.isTrustedCoreSourceToken(token)
}

// validateImplTarget checks nominal ownership, core authority and generic target syntax
// before any impl member or nested declaration can enter the semantic tables.
// Rules: rules/compiler/compiler_known_members.md — Lookup order;
// rules/declarations/impl.md — defining module and impl extensions; rules/types/temporal.md — §2.
func (a *Analyzer) validateImplTarget(impl *ast.ImplStatement) (Type, bool) {
	if impl == nil || impl.Target == nil {
		return Type{}, false
	}
	// A unit whose spelling an ordinary type of another module took keeps its
	// impl block in the unit's own module (rules/types/units.md).
	if unitEntry, shadowed := a.shadowedUnitTypes[impl.Target.Name]; shadowed && unitEntry.Module == a.currentModule {
		if unit, ok := a.units[impl.Target.Name]; ok {
			a.bindDefinition(impl.Target.Token, unit.Token)
		}
		a.shadowedUnitImpls[impl] = true
		return unitEntry, true
	}
	target, ok := a.types[impl.Target.Name]
	if !ok {
		a.addErrorAtToken(impl.Target.Token, "unknown impl target %s", impl.Target.Name)
		return Type{}, false
	}
	if definition, exists := a.typeDefinitionTokens[impl.Target.Name]; exists {
		a.bindDefinition(impl.Target.Token, definition)
	}
	// A real core declaration may refine a compiler-owned type into a named
	// struct. That representation detail never transfers extension authority.
	// Named user derivations retain their own identity and ordinary impl rules.
	// Rules: compiler_known_members.md — Lookup order; temporal.md — §2.
	if (target.Named || target.Kind == InvalidType) && compilerOwnedImplIdentity(target) && !a.isTrustedCoreSourceToken(impl.Target.Token) {
		a.addErrorAtToken(impl.Target.Token, "impl target %s is compiler-owned; only loader-proven core source may implement or extend it", impl.Target.Name)
		return Type{}, false
	}
	if !target.Named && target.Kind != InvalidType && !a.isAllowedCoreBuiltinImpl(impl.Target.Name, impl.Target.Token) {
		a.addErrorAtToken(impl.Target.Token, "impl target %s is not a named type", impl.Target.Name)
		return Type{}, false
	}
	if target.Kind == InterfaceType {
		a.addErrorAtToken(impl.Target.Token, "interface %s cannot have an ordinary impl block", impl.Target.Name)
		return Type{}, false
	}
	if !impl.Extends && target.Module != "" && a.currentModule != "" && target.Module != a.currentModule {
		a.addErrorAtToken(
			impl.Target.Token,
			"ordinary impl for %s must be declared in defining module %s, not module %s",
			typeDisplayName(target),
			moduleDisplayName(target.Module),
			moduleDisplayName(a.currentModule),
		)
		return Type{}, false
	}
	if !a.validateImplGenericTarget(impl, target) {
		return Type{}, false
	}
	return target, true
}

// isAllowedCoreBuiltinImpl grants builtin implementation authority only to
// loader-proven core source and the supported canonical builtin surface.
// Rules: rules/library/core-library.md — §1.2; compiler_known_members.md — Lookup order.
func (a *Analyzer) isAllowedCoreBuiltinImpl(target string, token lexer.Token) bool {
	if !a.isTrustedCoreSourceToken(token) {
		return false
	}
	return isCoreBuiltinImplTarget(target)
}

// isCoreBuiltinImplTarget lists the builtin families supported by privileged
// core impls; this compiler support list does not grant source provenance.
// Rules: rules/library/core-library.md — §1.2; rules/compiler/compiler_known_members.md — Lookup order.
func isCoreBuiltinImplTarget(target string) bool {
	switch target {
	case "bool",
		"byte",
		"char",
		"rune",
		"string",
		"int",
		"int8",
		"int16",
		"int32",
		"int64",
		"int128",
		"int256",
		"uint",
		"uint8",
		"uint16",
		"uint32",
		"uint64",
		"uint128",
		"uint256",
		"float",
		"float32",
		"float64",
		"decimal",
		"decimal128",
		"date",
		"time",
		"datetime",
		"duration",
		"RawPtr",
		"Option",
		"Result",
		// rules/collections/collections.md sections 1-2 and 13-15 make
		// these compiler-known collection families part of the core surface.
		// Only loader-proven core sources reach this allowlist; stdlib and user
		// modules therefore cannot use it to monkey-patch the families.
		"list",
		"map",
		"set",
		// rules/collections/shaped-types.md sections 34-35 reserve the
		// canonical shaped semantics for the compiler while permitting core
		// helpers and additive algorithms on these compiler-known identities.
		"vector",
		"matrix",
		"tensor",
		"tensor_view",
		"Shape",
		"Strides",
		"TensorLayout",
		"MemorySpace":
		return true
	default:
		return false
	}
}

// compilerOwnedImplIdentity recognizes the resolved canonical builtin identity,
// including a concrete core declaration, while excluding nominal user derivations.
// Rules: rules/compiler/compiler_known_members.md — Lookup order, Named and related types;
// rules/types/temporal.md — §2.
func compilerOwnedImplIdentity(target Type) bool {
	canonical, registered := sharedBuiltinTypes()[target.Name]
	return registered && canonical.Intrinsic && target.Intrinsic
}
