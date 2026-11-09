package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// threadLifecycleErrorVariants defines the closed portable creation/start
// failures. Unsupported thread capability belongs to static target validation.
// Rules: rules/concurrency/threads.md — §§18(1),19(1–3).
func threadLifecycleErrorVariants(name string) []string {
	switch name {
	case "ThreadSpawnError":
		return []string{"OutOfMemory", "ResourceLimit", "StackAllocationFailed", "InvalidConfiguration", "PermissionDenied", "ThreadLocalInitializationFailed", "NativeFailure"}
	case "ThreadStartError":
		return []string{"InvalidState", "ResourceUnavailable", "PermissionDenied", "NativeFailure"}
	}
	return nil
}

// builtinThreadLifecycleErrorType reserves the nominal core error identity
// and ordinary first-variant default without supplying runtime operations.
// Rules: rules/concurrency/threads.md — §§18–19;
// rules/types/default_values.md — "Enums".
func builtinThreadLifecycleErrorType(name string) Type {
	values := threadLifecycleErrorVariants(name)
	return Type{Name: name, Module: "core", Kind: EnumType,
		Named: true, Declared: true, Intrinsic: true, ErrorAssignable: true,
		Underlying: "int", EnumValues: values, EnumConsts: builtinEnumConsts(values), EnumDefault: values[0]}
}

// threadLifecycleErrorFromDeclaration refines trusted source only when it
// matches the complete canonical error enum, preserving declaration locations.
// Rules: rules/concurrency/threads.md — §§18–19;
// rules/library/core-library.md — compiler-known core declarations.
func (a *Analyzer) threadLifecycleErrorFromDeclaration(enum *ast.EnumDeclaration) (Type, bool) {
	name := enum.Name.Value
	if len(threadLifecycleErrorVariants(name)) == 0 || !a.isTrustedCoreBuiltinDeclaration(name, enum.Name.Token) {
		return Type{}, false
	}
	return a.exactCoreEnumDeclaration(enum, builtinThreadLifecycleErrorType(name), "rules/concurrency/threads.md §§18–19"), true
}

// rejectThreadLifecycleErrorNonEnum keeps aliases, aggregates and interfaces
// from replacing the reserved error identities even inside trusted core.
// Rules: rules/concurrency/threads.md — §§18–19.
func (a *Analyzer) rejectThreadLifecycleErrorNonEnum(name string, token lexer.Token) bool {
	if len(threadLifecycleErrorVariants(name)) == 0 || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "%s requires its canonical core error enum declaration", name)
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
