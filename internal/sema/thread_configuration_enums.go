package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// threadConfigurationVariants defines the portable start modes and relative
// priority intents, independently of native scheduler values or capabilities.
// Rules: rules/concurrency/threads.md — §§5(1–3),6(1–4).
func threadConfigurationVariants(name string) []string {
	switch name {
	case "ThreadStartMode":
		return []string{"Eager", "Deferred"}
	case "ThreadPriority":
		return []string{"Lowest", "Low", "Normal", "High", "Highest"}
	}
	return nil
}

// builtinThreadConfigurationEnumType reserves a nominal non-error core enum.
// Eager is the default start mode; priority's ordinary enum default is distinct
// from the ThreadSetting.Default used by a ThreadConfig initializer.
// Rules: rules/concurrency/threads.md — §§5–6,10;
// rules/types/default_values.md — "Enums".
func builtinThreadConfigurationEnumType(name string) Type {
	values := threadConfigurationVariants(name)
	return Type{Name: name, Module: "core", Kind: EnumType, Named: true,
		Declared: true, Intrinsic: true, Underlying: "int", EnumValues: values,
		EnumConsts: builtinEnumConsts(values), EnumDefault: values[0]}
}

// threadConfigurationEnumFromDeclaration requires the exact non-error core
// declaration and retains source/member tokens for ordinary tooling navigation.
// Rules: rules/concurrency/threads.md — §§5–6;
// rules/library/core-library.md — compiler-known core declarations.
func (a *Analyzer) threadConfigurationEnumFromDeclaration(enum *ast.EnumDeclaration) (Type, bool) {
	name := enum.Name.Value
	if len(threadConfigurationVariants(name)) == 0 || !a.isTrustedCoreBuiltinDeclaration(name, enum.Name.Token) {
		return Type{}, false
	}
	return a.exactCoreEnumDeclaration(enum, builtinThreadConfigurationEnumType(name), "rules/concurrency/threads.md §§5–6"), true
}

// rejectThreadConfigurationNonEnum prevents trusted core from replacing the
// closed configuration enums with aliases, aggregates or interfaces.
// Rules: rules/concurrency/threads.md — §§5–6.
func (a *Analyzer) rejectThreadConfigurationNonEnum(name string, token lexer.Token) bool {
	if len(threadConfigurationVariants(name)) == 0 || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "%s requires its canonical core enum declaration", name)
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
