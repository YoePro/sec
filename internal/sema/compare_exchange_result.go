package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// builtinCompareExchangeResultType reserves the exact nominal core enum.
// The existing sum-type machinery carries its discriminant and associated T;
// AssociatedEnum preserves the source declaration kind. It is never an error.
// Rules: rules/concurrency/atomics.md — §§4,6;
// rules/types/default_values.md — Enums (first declared member).
func builtinCompareExchangeResultType() Type {
	payload := Type{Name: "T", Kind: GenericType}
	return Type{Name: "CompareExchangeResult", Module: "core", Kind: UnionType,
		Named: true, Declared: true, Intrinsic: true, AssociatedEnum: true,
		GenericParameters: []string{"T"}, EnumValues: []string{"Exchanged", "NotExchanged"},
		UnionVariants: []UnionVariant{{Name: "Exchanged"}, {Name: "NotExchanged", Payload: &payload}},
		UnionDefault:  "Exchanged"}
}

// compareExchangeResultType closes the standalone enum identity for member
// metadata, retaining the exact nominal payload type without a struct stub.
// Rules: rules/concurrency/atomics.md — §§6(2–8),12,17.
func compareExchangeResultType(element Type) Type {
	typ := builtinCompareExchangeResultType()
	typ.GenericParameters = nil
	typ.TypeArgs = []Type{element}
	typ.UnionVariants[1].Payload = &element
	return typ
}

// isCompareExchangeResult identifies the canonical associated-value enum,
// independently of template or concrete generic substitution.
// Rules: rules/concurrency/atomics.md — §§4,6.
func isCompareExchangeResult(typ Type) bool {
	return typ.Intrinsic && typ.Named && typ.Module == "core" && typ.Name == "CompareExchangeResult" && typ.AssociatedEnum && typ.Kind == UnionType
}

// CompareExchangeResultType reports the compiler-owned enum shape for tooling;
// ordinary unions and same-spelled project declarations do not acquire it.
// Rules: rules/concurrency/atomics.md — §§4,6,43(2),44(3–5).
func CompareExchangeResultType(typ Type) (Type, bool) {
	typ = dereferenceType(typ)
	return typ, isCompareExchangeResult(typ)
}

// compareExchangeResultFromDeclaration refines the reserved identity only from
// the canonical generic enum in loader-proven core, preserving source tokens.
// Rules: rules/concurrency/atomics.md — §§4(4–7),6(1–8),43–45.
func (a *Analyzer) compareExchangeResultFromDeclaration(enum *ast.EnumDeclaration) (Type, bool) {
	if enum.Name.Value != "CompareExchangeResult" || !a.isTrustedCoreBuiltinDeclaration(enum.Name.Value, enum.Name.Token) {
		return Type{}, false
	}
	typ := builtinCompareExchangeResultType()
	typ.DeclarationToken = enum.Name.Token
	valid := !enum.ErrorType && !enum.BitUnderlying && enum.UnderlyingType == nil && len(enum.Attributes) == 0 && len(enum.GenericParameters) == 1 && len(enum.Values) == 2
	if len(enum.GenericParameters) == 1 {
		parameter := enum.GenericParameters[0]
		valid = valid && parameter != nil && parameter.Name != nil && parameter.Name.Value == "T" && len(parameter.Constraints) == 0
	}
	if valid {
		for index, value := range enum.Values {
			if value == nil || value.Invalid || value.Name == nil || value.Name.Value != typ.UnionVariants[index].Name || value.Default || value.Initializer != nil {
				valid = false
				break
			}
			if index == 0 {
				valid = value.Payload == nil
			} else if value.Payload == nil {
				valid = false
			} else {
				payload, ok := a.resolveType(value.Payload)
				valid = ok && sameConcreteType(payload, Type{Name: "T", Kind: GenericType})
			}
			if !valid {
				break
			}
			typ.UnionVariants[index].Token = value.Name.Token
			a.recordDefinition(value.Name.Token)
		}
	}
	if !valid {
		a.addErrorAtToken(enum.Name.Token, "CompareExchangeResult requires its canonical core enum declaration: enum CompareExchangeResult[T] { Exchanged, NotExchanged(T) }")
	}
	return typ, true
}

// rejectCompareExchangeResultNonEnum keeps even trusted core from replacing
// the canonical enum with a struct, union, scalar or interface declaration.
// Rules: rules/concurrency/atomics.md — §§4(7),6(1–8).
func (a *Analyzer) rejectCompareExchangeResultNonEnum(name string, token lexer.Token) bool {
	if name != "CompareExchangeResult" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "CompareExchangeResult requires its canonical associated-value enum declaration")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
