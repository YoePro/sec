package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// genericTypePackName retains the declaration's trailing ordered type sequence
// independently from ordinary single-type generic parameters.
// Rules: rules/declarations/generics.md — §16A(1–2); MD-117 — §2(5).
func genericTypePackName(parameters []*ast.GenericParameter) string {
	for _, parameter := range parameters {
		if parameter != nil && parameter.TypePack && parameter.Name != nil {
			return parameter.Name.Value
		}
	}
	return ""
}

// rejectUnavailableTypePackExpansion keeps recognized declaration metadata
// available to tooling. Shared packs unused or observed only through Len can
// be checked; other modes and bodies retain the concrete expansion gate.
// A type sequence must never silently acquire the meaning of one generic T.
// Rules: rules/declarations/generics.md — §16A(1–5);
// rules/corrections/md117-heterogeneous-variadic-type-packs-correction.md — §§2,9–10.
func (a *Analyzer) rejectUnavailableTypePackExpansion(fn *ast.FunctionDeclaration) bool {
	if a.supportedSharedTypePackBody(fn) {
		return false
	}
	for _, parameter := range fn.GenericParameters {
		if parameter != nil && parameter.TypePack {
			a.addErrorAtTokenWithMetadata(parameter.PackToken, diagnostics.GenericTypePackExpansionUnavailable,
				"The declaration is recognized. Shared-pack bodies with no pack use or only Len reads and direct-call inference are supported. This parameter mode or body needs heterogeneous expansion that is not implemented yet; no runtime container or ordinary single-type parameter is substituted.",
				"compiler capability unavailable: semantic expansion of heterogeneous type pack %s", parameter.Name.Value)
			return true
		}
	}
	return false
}

// isDeclaredTypePackMode recognizes only the declared sequence's pointwise
// owned/shared/mutable mode, without relaxing homogeneous variadic restrictions.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §§1(4),3(1–6).
func isDeclaredTypePackMode(typ Type, name string) bool {
	if typ.Kind == ReferenceType && typ.Element != nil {
		typ = *typ.Element
	}
	return name != "" && typ.Kind == GenericTypePackType && typ.Name == name
}
