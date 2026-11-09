package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinMessageDispositionType defines exactly the portable unavailable
// outcomes, independently of sender lifetime or target-specific close causes.
// Rules: rules/concurrency/channels.md — §13(1–4),§14(3).
func builtinMessageDispositionType() Type {
	values := []string{"Received", "Expired", "Discarded"}
	return Type{Name: "MessageDisposition", Module: "core", Kind: EnumType,
		Named: true, Declared: true, Intrinsic: true, Underlying: "int",
		EnumValues: values, EnumConsts: builtinEnumConsts(values), EnumDefault: values[0]}
}

// builtinChannelRevokeResultType retains both the returned owning payload and
// the complete nominal disposition enum, rather than a name-only placeholder.
// Rules: rules/concurrency/channels.md — §14(1–3).
func builtinChannelRevokeResultType() Type {
	payload := Type{Name: "T", Kind: GenericType}
	disposition := builtinMessageDispositionType()
	return Type{Name: "ChannelRevokeResult", Module: "core", Kind: UnionType,
		Named: true, Declared: true, Intrinsic: true, GenericParameters: []string{"T"},
		UnionVariants: []UnionVariant{{Name: "Revoked", Payload: &payload}, {Name: "Unavailable", Payload: &disposition}}}
}

// registerChannelRevokeDeclaration requires the exact trusted result union;
// the disposition identity must be declared as an enum rather than an alias.
// Rules: rules/concurrency/channels.md — §§13–14.
func (a *Analyzer) registerChannelRevokeDeclaration(stmt *ast.TypeDeclStatement) bool {
	name := stmt.Name.Value
	if (name != "ChannelRevokeResult" && name != "MessageDisposition") || !a.isTrustedCoreBuiltinDeclaration(name, stmt.Name.Token) {
		return false
	}
	if name == "MessageDisposition" {
		a.addErrorAtToken(stmt.Name.Token, "MessageDisposition requires its canonical core enum declaration")
		a.types[name] = builtinMessageDispositionType()
		return true
	}
	a.types[name] = a.exactCoreUnionDeclaration(stmt, builtinChannelRevokeResultType(), "rules/concurrency/channels.md §14")
	return true
}

// rejectChannelRevokeNonUnion prevents enum/interface replacement of the
// canonical generic result even when the declaration is in trusted core.
// Rules: rules/concurrency/channels.md — §14(1–2).
func (a *Analyzer) rejectChannelRevokeNonUnion(name string, token lexer.Token) bool {
	if name != "ChannelRevokeResult" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "ChannelRevokeResult requires its canonical core union declaration")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}

// rejectMessageDispositionInterface keeps the closed portable enum identity
// from being replaced by a source interface.
// Rules: rules/concurrency/channels.md — §13(1–4).
func (a *Analyzer) rejectMessageDispositionInterface(name string, token lexer.Token) bool {
	if name != "MessageDisposition" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "MessageDisposition requires its canonical core enum declaration")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
