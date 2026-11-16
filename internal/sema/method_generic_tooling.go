package sema

import "sec/internal/ast"

// MethodTypeArgument is one declaration-ordered, resolved owner or method
// binding. Its type is a detached compiler fact, not LSP inference.
// Rules: rules/declarations/generics.md — §§9,18–20,30;
// rules/tooling/lsp.md — §12(1),(3).
type MethodTypeArgument struct {
	Name string
	Type Type
}

// ResolvedMethodTypeArguments keeps the enclosing and method-local parameter
// roles separate at the exact selected call. Symbolic template calls may have
// bindings without a complete concrete specialization identity.
// Rules: rules/compiler/monomorphization.md — §§4,6.4;
// rules/declarations/generics.md — §9.
type ResolvedMethodTypeArguments struct {
	Owner    []MethodTypeArgument
	Method   []MethodTypeArgument
	Identity string
}

// ResolvedMethodTypeArgumentsOf exposes only a successful compiler-selected
// method, preserving declaration order and the fixed owner environment. It
// never chooses an overload or creates a specialization for a tooling query.
// Rules: rules/declarations/generics.md — §§9,18–20,29–30;
// rules/compiler/monomorphization.md — §§6.3–6.4;
// rules/tooling/lsp.md — §12(1),(3).
func (a *Analyzer) ResolvedMethodTypeArgumentsOf(call *ast.CallExpression) (ResolvedMethodTypeArguments, bool) {
	if a == nil {
		return ResolvedMethodTypeArguments{}, false
	}
	target, known := a.ResolvedCallTarget(call)
	if !known || target.Function.ImplTarget == "" {
		return ResolvedMethodTypeArguments{}, false
	}
	template, known := a.genericCallableTemplate(target.Function)
	if !known || len(template.GenericParameters) == 0 || template.GenericTypePack != "" {
		return ResolvedMethodTypeArguments{}, false
	}
	arguments := func(names []string) ([]MethodTypeArgument, bool) {
		result := make([]MethodTypeArgument, len(names))
		for index, name := range names {
			typ, known := target.Function.GenericSubstitution[name]
			if !known {
				return nil, false
			}
			result[index] = MethodTypeArgument{Name: name, Type: semanticSnapshotType(typ)}
		}
		return result, true
	}
	fact := ResolvedMethodTypeArguments{Identity: target.Function.InstantiationIdentity}
	fact.Owner, known = arguments(a.types[template.ImplTarget].GenericParameters)
	if !known {
		return ResolvedMethodTypeArguments{}, false
	}
	fact.Method, known = arguments(template.GenericParameters)
	if !known {
		return ResolvedMethodTypeArguments{}, false
	}
	return fact, true
}
