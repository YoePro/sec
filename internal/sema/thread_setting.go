package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinThreadSettingType provides the canonical ordinary generic union.
// Construction and assignment use normal union rules, with no T-to-Preferred
// coercion or configuration-specific ownership behavior.
// Rules: rules/concurrency/threads.md — §4(1–4).
func builtinThreadSettingType() Type {
	payload := Type{Name: "T", Kind: GenericType}
	return Type{Name: "ThreadSetting", Module: "core", Kind: UnionType,
		Named: true, Declared: true, Intrinsic: true, GenericParameters: []string{"T"},
		UnionVariants: []UnionVariant{{Name: "Default"}, {Name: "Preferred", Payload: &payload}, {Name: "Required", Payload: &payload}}}
}

// registerThreadSettingDeclaration validates the complete trusted core union
// shape and retains real declaration/member identities for tooling.
// Rules: rules/concurrency/threads.md — §4(1–4);
// rules/library/core-library.md — compiler-known core declarations.
func (a *Analyzer) registerThreadSettingDeclaration(stmt *ast.TypeDeclStatement) bool {
	if stmt.Name.Value != "ThreadSetting" || !a.isTrustedCoreBuiltinDeclaration(stmt.Name.Value, stmt.Name.Token) {
		return false
	}
	a.types["ThreadSetting"] = a.exactCoreUnionDeclaration(stmt, builtinThreadSettingType(), "rules/concurrency/threads.md §4")
	return true
}

// rejectThreadSettingNonUnion keeps an enum/interface declaration from
// replacing the compiler-known generic configuration union, even in core.
// Rules: rules/concurrency/threads.md — §4(1).
func (a *Analyzer) rejectThreadSettingNonUnion(name string, token lexer.Token) bool {
	if name != "ThreadSetting" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "ThreadSetting requires its canonical core union declaration")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
