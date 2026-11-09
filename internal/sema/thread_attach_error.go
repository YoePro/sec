package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinThreadAttachErrorType reserves the closed portable attachment error
// identity, independently of attachment runtime availability.
// Rules: rules/concurrency/thread_local.md — §29(1–3);
// rules/types/default_values.md — "Enums".
func builtinThreadAttachErrorType() Type {
	values := []string{"AlreadyAttached", "ResourceUnavailable", "NativeFailure"}
	return Type{Name: "ThreadAttachError", Module: "core", Kind: EnumType,
		Named: true, Declared: true, Intrinsic: true, ErrorAssignable: true,
		Underlying: "int", EnumValues: values, EnumConsts: builtinEnumConsts(values),
		EnumDefault: values[0]}
}

// threadAttachErrorFromDeclaration retains trusted source locations while
// requiring the exact error enum, including its error marker and member order.
// Rules: rules/concurrency/thread_local.md — §29(1–3);
// rules/library/core-library.md — compiler-known core declarations.
func (a *Analyzer) threadAttachErrorFromDeclaration(enum *ast.EnumDeclaration) (Type, bool) {
	if enum.Name.Value != "ThreadAttachError" || !a.isTrustedCoreBuiltinDeclaration(enum.Name.Value, enum.Name.Token) {
		return Type{}, false
	}
	return a.exactCoreEnumDeclaration(enum, builtinThreadAttachErrorType(), "rules/concurrency/thread_local.md §29"), true
}

// rejectThreadAttachErrorNonEnum prevents trusted source from replacing the
// canonical attachment error with a struct, alias, union or interface.
// Rules: rules/concurrency/thread_local.md — §29(1–3).
func (a *Analyzer) rejectThreadAttachErrorNonEnum(name string, token lexer.Token) bool {
	if name != "ThreadAttachError" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "ThreadAttachError requires its canonical core error enum declaration")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
