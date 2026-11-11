package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinContextType reserves the opaque standalone cancellation-context owner.
// Runtime state and cancellation authority are not ordinary public fields.
// Rules: rules/concurrency/mutex.md — §§4,8; rules/concurrency/cancellation.md — Context.
func builtinContextType() Type {
	return Type{Name: "Context", Module: "core", Kind: StructType, Named: true, Declared: true, Intrinsic: true, ExplicitlyNonCopyable: true, NoCopyPolicyOrigin: "Context"}
}

// builtinContextSourceType reserves the opaque owner of a child Context and
// its cancellation authority; the public surface never exposes owned extraction.
// Rules: rules/concurrency/cancellation.md — §§36,38.
func builtinContextSourceType() Type {
	return Type{Name: "ContextSource", Module: "core", Kind: StructType, Named: true, Declared: true, Intrinsic: true, ExplicitlyNonCopyable: true, NoCopyPolicyOrigin: "ContextSource"}
}

// isCancellationContextType recognizes only the canonical nominal core owners.
// Rules: rules/concurrency/cancellation.md — §§35–38.
func isCancellationContextType(typ Type) bool {
	typ = dereferenceType(typ)
	return typ.Kind == StructType && typ.Module == "core" && typ.Intrinsic && (typ.Name == "Context" || typ.Name == "ContextSource")
}

// builtinContextErrorType preserves the exact operation-local error family,
// distinct from terminal cancellation of the current execution entity.
// Rules: rules/concurrency/mutex.md — §§7,24(2–6).
func builtinContextErrorType() Type {
	values := []string{"Cancelled", "DeadlineExceeded"}
	return Type{Name: "ContextError", Module: "core", Kind: EnumType, Named: true, Declared: true, Underlying: "int", ErrorAssignable: true, EnumValues: values, EnumConsts: builtinEnumConsts(values), EnumDefault: "Cancelled"}
}

// registerContextDeclaration refines the canonical opaque Context identity
// only from the exact @noCopy source declaration in loader-proven core.
// Rules: rules/concurrency/mutex.md — §§4,8;
// rules/concurrency/cancellation.md — §§35–36.
func (a *Analyzer) registerContextDeclaration(stmt *ast.TypeDeclStatement) bool {
	name := stmt.Name.Value
	if (name != "Context" && name != "ContextSource") || !a.isTrustedCoreBuiltinDeclaration(name, stmt.Name.Token) {
		return false
	}
	if stmt.BaseType != nil || stmt.AssignedType != nil || stmt.StructType != nil || stmt.RegisterType != nil || stmt.Union || len(stmt.Variants) != 0 || stmt.ErrorType || stmt.Contract != nil || stmt.Default != nil || len(stmt.Implements) != 0 || len(stmt.GenericParameters) != 0 || len(stmt.Attributes) != 1 || !hasAttribute(stmt.Attributes, "noCopy") {
		a.addErrorAtToken(stmt.Name.Token, "%s requires its canonical opaque declaration: @noCopy type %s", name, name)
	}
	typ := builtinContextType()
	if name == "ContextSource" {
		typ = builtinContextSourceType()
	}
	typ.DeclarationToken = stmt.Name.Token
	a.types[name] = typ
	return true
}

// rejectContextNonOpaqueDeclaration keeps runtime context state opaque even
// when a malformed enum/interface declaration originates in trusted core.
// Rules: rules/concurrency/mutex.md — §§4(4–5),8;
// rules/concurrency/cancellation.md — §§35–36.
func (a *Analyzer) rejectContextNonOpaqueDeclaration(name string, token lexer.Token) bool {
	if (name != "Context" && name != "ContextSource") || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "%s requires its canonical opaque declaration: @noCopy type %s", name, name)
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
