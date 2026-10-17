package sema

import (
	"fmt"
	"math/big"
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
// Rules: rules/library/core-library.md; rules/concurrency/mutex.md §§5–8,13;
// rules/types/contracts.md "Public conversion and contract errors" (ContractError, MD-012).
// rules/concurrency/atomics.md — §§4–6 (MemoryOrder, CompareExchangeResult).
// rules/concurrency/channels.md — §§4–5 (ChannelCapability).
func isCoreBuiltinDeclaration(name string) bool {
	// rules/concurrency/ipc.md — §§6,27,44,55,68,79,94;
	// rules/concurrency/processes.md — §§5.12–5.13.
	if len(ipcErrorVariants(name)) != 0 || len(commandIOModeVariants(name)) != 0 {
		return true
	}
	switch name {
	case "IndexError", "FormatError", "StringError", "ContractError", "TaskOutcome", "TaskSpawnError", "TaskError",
		"date", "time", "datetime", "duration", "Instant", "MemoryOrder", "CompareExchangeResult", "ChannelCapability", "Mutex", "MutexGuard", "Context", "ContextError":
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
	// MD-040: `impl <Unit>` targets only the unit, in the unit's own module;
	// `impl Name` targets only an ordinary type.
	if impl.Target.UnitOnly {
		unitEntry, declared := a.shadowedUnitTypes[impl.Target.Name]
		unit, known := a.units[impl.Target.Name]
		if !declared || !known || impl.Target.UnitExpression == nil || impl.Target.UnitExpression.Kind != ast.UnitExpressionName {
			a.addErrorAtToken(impl.Target.Token, "unknown unit impl target <%s>; impl <Unit> names one declared unit", impl.Target.Unit)
			return Type{}, false
		}
		if unitEntry.Module != a.currentModule {
			a.addErrorAtToken(impl.Target.Token, "impl <%s> must be in module %s, which declares the unit", impl.Target.Name, moduleDisplayName(unitEntry.Module))
			return Type{}, false
		}
		a.bindDefinition(impl.Target.Token, unit.Token)
		a.shadowedUnitImpls[impl] = true
		return unitEntry, true
	}
	target, ok := a.types[impl.Target.Name]
	if !ok {
		if _, isUnit := a.units[impl.Target.Name]; isUnit {
			a.addErrorAtTokenWithMetadata(impl.Target.Token, "",
				"A unit is always referenced as <Unit> (MD-040); write impl <"+impl.Target.Name+"> for its metadata and conversion functions.",
				"impl %s does not name an ordinary type; %s is a unit", impl.Target.Name, impl.Target.Name)
			return Type{}, false
		}
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

// typeFromEnumDeclaration resolves ordinary enum carriers, constants and defaults
// while retaining exact trusted compiler-known core declarations.
// Rules: rules/declarations/enums.md — §§3–6,8–10;
// rules/types/default_values.md — Enums;
// rules/concurrency/atomics.md — §§4–6; rules/concurrency/ipc.md — §§6,27,44,55,68,79,94;
// rules/concurrency/processes.md — §§5.12–5.13.
func (a *Analyzer) typeFromEnumDeclaration(name string, enum *ast.EnumDeclaration) Type {
	if typ, canonical := a.ipcCoreEnumFromDeclaration(enum); canonical {
		return typ
	}
	if typ, canonical := a.compareExchangeResultFromDeclaration(enum); canonical {
		return typ
	}
	underlying := a.types["int"]
	if enum.BitUnderlying {
		width := enum.UnderlyingBitWidth
		if width <= 0 || width > 256 {
			a.addErrorAtToken(enum.Name.Token, "enum %s bit width must be between 1 and 256, got %d", name, width)
			return Type{Name: name, Kind: InvalidType, Named: true, Declared: true}
		}
		max := new(big.Int).Lsh(big.NewInt(1), uint(width))
		max.Sub(max, big.NewInt(1))
		underlying = Type{
			Name:       fmt.Sprintf("bit[%d]", width),
			Kind:       UintType,
			MinInteger: big.NewInt(0),
			MaxInteger: max,
		}
	} else if enum.UnderlyingType != nil {
		resolved, ok := a.resolveType(enum.UnderlyingType)
		if !ok {
			return Type{Name: name, Kind: InvalidType}
		}
		underlying = resolved
	}

	if underlying.Kind != IntType && underlying.Kind != UintType && underlying.Kind != StringType {
		token := enum.Name.Token
		if enum.UnderlyingType != nil {
			token = enum.UnderlyingType.Token
		}
		a.addErrorAtToken(token, "enum %s underlying type must be integer or string, got %s", name, typeDisplayName(underlying))
		return Type{Name: name, Kind: InvalidType}
	}

	noCopy := hasAttribute(enum.Attributes, "noCopy")
	typ := Type{
		Name:                  name,
		Module:                a.currentModule,
		Kind:                  EnumType,
		Named:                 true,
		Declared:              true,
		ExplicitlyNonCopyable: noCopy,
		ErrorAssignable:       enum.ErrorType,
		Underlying:            underlying.Name,
		GenericParameters:     genericParameterNameValues(enum.GenericParameters),
		GenericConstraints:    a.resolvedGenericParameterConstraints(enum.GenericParameters),
		EnumConsts:            map[string]EnumValue{},
	}
	// Rules: rules/concurrency/atomics.md — §§4–5 and
	// rules/concurrency/channels.md — §§4–5: source and compiler
	// registration are one reserved core identity.
	if a.isTrustedCoreBuiltinDeclaration(name, enum.Name.Token) {
		typ.Intrinsic = true
	}
	if underlying.MinInteger != nil {
		typ.MinInteger = new(big.Int).Set(underlying.MinInteger)
	}
	if underlying.MaxInteger != nil {
		typ.MaxInteger = new(big.Int).Set(underlying.MaxInteger)
	}
	if noCopy {
		typ.NoCopyPolicyOrigin = name
	}
	if enum.BitUnderlying {
		typ.BitWidth = enum.UnderlyingBitWidth
	}

	seen := map[string]lexer.Token{}
	var repeatedInitializer ast.Expression
	var explicitDefaultToken lexer.Token
	semanticIndex := 0
	for _, value := range enum.Values {
		if value == nil || value.Invalid || value.Name == nil {
			continue
		}
		i := semanticIndex
		semanticIndex++
		if previousToken, exists := seen[value.Name.Value]; exists {
			a.addErrorAtTokenWithPrevious(value.Token, previousToken, "duplicate enum value %q in enum %s", value.Name.Value, name)
			continue
		}
		a.checkConfusableInDomains(value.Name.Value, value.Name.Token, seen)
		seen[value.Name.Value] = value.Token

		if value.Default {
			if typ.EnumDefault != "" {
				a.addErrorAtTokenWithPrevious(value.DefaultToken, explicitDefaultToken, "enum %s may only mark one member default", name)
			} else {
				typ.EnumDefault = value.Name.Value
				explicitDefaultToken = value.DefaultToken
			}
		}

		initializer := value.Initializer
		if underlying.Kind == StringType {
			if initializer == nil {
				a.addErrorAtToken(value.Token, "string-backed enum member %s.%s requires an explicit initializer", name, value.Name.Value)
				continue
			}
			literal, ok := initializer.(*ast.StringLiteral)
			if !ok {
				if identifier, isIdentifier := initializer.(*ast.Identifier); isIdentifier && identifier.Value == "iota" {
					a.addErrorAtToken(identifier.Token, "iota is not available in string-backed enum %s", name)
					continue
				}
				a.addErrorAtToken(expressionToken(initializer), "string-backed enum value %s.%s initializer must be a compile-time string constant", name, value.Name.Value)
				continue
			}
			stringValue := literal.Value
			typ.EnumValues = append(typ.EnumValues, value.Name.Value)
			typ.EnumConsts[value.Name.Value] = EnumValue{
				Name:        value.Name.Value,
				StringValue: &stringValue,
				Token:       value.Token,
			}
			a.recordDefinition(value.Name.Token)
			continue
		}
		if initializer != nil {
			repeatedInitializer = initializer
		} else if repeatedInitializer != nil {
			initializer = repeatedInitializer
		} else {
			initializer = &ast.Identifier{Token: value.Token, Value: "iota"}
			repeatedInitializer = initializer
		}
		next, ok := a.enumInitializerIntegerValue(initializer, i)
		if !ok {
			a.addErrorAtToken(expressionToken(initializer), "enum value %s.%s initializer must be integer constant", name, value.Name.Value)
			continue
		}

		a.checkIntegerValueRange(underlying, next, value.Token)
		typ.EnumValues = append(typ.EnumValues, value.Name.Value)
		typ.EnumConsts[value.Name.Value] = EnumValue{
			Name:  value.Name.Value,
			Value: new(big.Int).Set(next),
			Token: value.Token,
		}
		a.recordDefinition(value.Name.Token)
	}
	if typ.EnumDefault == "" && len(typ.EnumValues) > 0 {
		typ.EnumDefault = typ.EnumValues[0]
	}

	return typ
}

// enumInitializerIntegerValue evaluates an enum initializer with its lexical
// iota index and restores the surrounding compile-time integer environment.
// Rules: rules/declarations/enums.md — §§5–6.2.
func (a *Analyzer) enumInitializerIntegerValue(expr ast.Expression, iotaIndex int) (*big.Int, bool) {
	previous, hadPrevious := a.constInts["iota"]
	a.constInts["iota"] = big.NewInt(int64(iotaIndex))
	defer func() {
		if hadPrevious {
			a.constInts["iota"] = previous
			return
		}
		delete(a.constInts, "iota")
	}()

	return a.integerConstantValue(expr)
}
