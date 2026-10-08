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
