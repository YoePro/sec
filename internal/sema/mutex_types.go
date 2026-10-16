package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinMutexType installs the canonical opaque, non-copyable owner template.
// Runtime storage is intentionally absent from the public field surface.
// Rules: rules/concurrency/mutex.md — §§4–6.
func builtinMutexType(name string) Type {
	return Type{Name: name, Module: "core", Kind: StructType, Named: true,
		Declared: true, Intrinsic: true, GenericParameters: []string{"T"},
		ExplicitlyNonCopyable: true, NoCopyPolicyOrigin: name}
}

// mutexGuardType binds the protected type without losing canonical ownership
// and opaque-storage traits when a basic acquisition produces a guard.
// Rules: rules/concurrency/mutex.md — §§6,20–22.
func mutexGuardType(inner Type) Type {
	typ := builtinMutexType("MutexGuard")
	typ.GenericParameters = nil
	typ.TypeArgs = []Type{inner}
	return typ
}

// registerMutexDeclaration accepts only the exact opaque @noCopy declaration
// in loader-proven core source; ordinary user declarations remain reserved.
// Rules: rules/concurrency/mutex.md — §§4–6.
func (a *Analyzer) registerMutexDeclaration(stmt *ast.TypeDeclStatement) bool {
	name := stmt.Name.Value
	if (name != "Mutex" && name != "MutexGuard") || !a.isTrustedCoreBuiltinDeclaration(name, stmt.Name.Token) {
		return false
	}
	valid := stmt.BaseType == nil && stmt.AssignedType == nil && stmt.StructType == nil && stmt.RegisterType == nil && !stmt.Union && len(stmt.Variants) == 0 && !stmt.ErrorType && stmt.Contract == nil && stmt.Default == nil && len(stmt.Implements) == 0 && len(stmt.GenericParameters) == 1 && len(stmt.Attributes) == 1 && hasAttribute(stmt.Attributes, "noCopy")
	if len(stmt.GenericParameters) == 1 {
		parameter := stmt.GenericParameters[0]
		valid = valid && parameter != nil && parameter.Name != nil && parameter.Name.Value == "T" && len(parameter.Constraints) == 0
	}
	if !valid {
		a.addErrorAtToken(stmt.Name.Token, "%s requires its canonical opaque declaration: @noCopy type %s[T]", name, name)
	}
	typ := builtinMutexType(name)
	typ.DeclarationToken = stmt.Name.Token
	a.types[name] = typ
	return true
}

// rejectMutexNonOpaqueDeclaration prevents trusted core enum/interface syntax
// from replacing the compiler-owned synchronization/acquisition identity.
// Rules: rules/concurrency/mutex.md — §§4(4–5),5–6.
func (a *Analyzer) rejectMutexNonOpaqueDeclaration(name string, token lexer.Token) bool {
	if (name != "Mutex" && name != "MutexGuard") || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "%s requires its canonical opaque declaration: @noCopy type %s[T]", name, name)
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
